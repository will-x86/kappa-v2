package cont

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	testNamespace    = "kappa-v2-cont-test"
	testImageAlpine  = "docker.io/library/alpine:latest"
	containerdSocket = "/run/containerd/containerd.sock"
)

// setupContainerdTest checks if containerd is accessible and sets up the namespace.
func setupContainerdTest(t *testing.T) context.Context {
	t.Helper()
	if _, err := os.Stat(containerdSocket); os.IsNotExist(err) {
		t.Skipf("containerd socket not found at %s, skipping integration test", containerdSocket)
	}

	client, err := containerd.New(containerdSocket)
	require.NoError(t, err, "Failed to connect to containerd for setup")
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), testNamespace)
	logger := zap.NewNop() // Use Nop logger for test setup clarity
	logger.Info("Using test namespace", zap.String("namespace", testNamespace))

	return ctx
}

func TestNewContainer_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  ContainerConfig
		wantErr bool
	}{
		{"valid config", ContainerConfig{Image: "img", Name: "name", Namespace: "ns", Command: []string{"cmd"}, Env: []string{}}, false},
		{"missing image", ContainerConfig{Name: "name", Namespace: "ns", Command: []string{"cmd"}, Env: []string{}}, true},
		{"missing name", ContainerConfig{Image: "img", Namespace: "ns", Command: []string{"cmd"}, Env: []string{}}, true},
		// Namespace defaults, so not strictly required by validator if "" is allowed and defaulted
		{"missing command", ContainerConfig{Image: "img", Name: "name", Namespace: "ns", Env: []string{}}, true},
		// Env can be empty
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewContainer(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainer_Lifecycle_Simple(t *testing.T) {
	setupContainerdTest(t) // Skips if no containerd

	containerName := "test-lifecycle-" + uuid.NewString()
	cfg := ContainerConfig{
		Image:     testImageAlpine,
		Name:      containerName,
		Namespace: testNamespace,
		Command:   []string{"sh", "-c", "echo 'hello from container' && exit 0"},
		Env:       []string{"TEST_ENV=1"},
		RemoveOptions: RemoveOptions{
			RemoveContainerIfExists: true,
			RemoveSnapshotIfExists:  true,
		},
	}

	c, err := NewContainer(cfg)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, c.Close(), "Container Close() failed")
	}()
	// Defer remove just in case test fails mid-way
	defer func() {
		stopOpts := StopOptions{Timeout: 5 * time.Second, ForceKill: true, RemoveOnStop: true}
		_ = c.Stop(stopOpts) // Try to stop and remove
		_ = c.Remove()       // Ensure removal
	}()

	err = c.Start()
	require.NoError(t, err, "Container Start() failed")

	// Wait for task to complete
	statusC, err := c.task.Wait(c.ctx)
	require.NoError(t, err, "Failed to wait for task")

	select {
	case status := <-statusC:
		assert.True(t, status.ExitCode() == 0 || status.ExitCode() == 137 || status.ExitCode() == 143, "Container exited with non-zero/non-term/non-kill code: %d", status.ExitCode())
	case <-time.After(15 * time.Second): // Increased timeout
		t.Fatal("Container did not exit in time")
	}

	logs := c.GetLogs()
	foundLog := false
	for _, logLine := range logs {
		if strings.Contains(logLine, "hello from container") {
			foundLog = true
			break
		}
	}
	assert.True(t, foundLog, "Expected log message not found in container logs: %v", logs)

	stopOpts := StopOptions{Timeout: 5 * time.Second, ForceKill: false, RemoveOnStop: true}
	err = c.Stop(stopOpts) // Should be already stopped, but this triggers removal
	require.NoError(t, err, "Container Stop() with RemoveOnStop failed")

	// Verify it's removed (loading it should fail)
	client, _ := containerd.New(containerdSocket)
	defer client.Close()
	_, err = client.LoadContainer(namespaces.WithNamespace(context.Background(), testNamespace), containerName)
	assert.Error(t, err, "Container should be removed and not loadable")
}

func TestContainer_LogStreaming(t *testing.T) {
	setupContainerdTest(t)
	containerName := "test-logstream-" + uuid.NewString()
	cfg := ContainerConfig{
		Image:     testImageAlpine,
		Name:      containerName,
		Namespace: testNamespace,
		Command:   []string{"sh", "-c", "echo 'stdout_message_stream'; echo 'stderr_message_stream' >&2; sleep 0.2; exit 0"},
		Env:       []string{},
		RemoveOptions: RemoveOptions{
			RemoveContainerIfExists: true,
			RemoveSnapshotIfExists:  true,
		},
	}

	c, err := NewContainer(cfg)
	require.NoError(t, err)
	defer c.Close()
	defer func() {
		stopOpts := StopOptions{Timeout: 5 * time.Second, ForceKill: true, RemoveOnStop: true}
		_ = c.Stop(stopOpts)
		_ = c.Remove()
	}()

	var streamedLogs []string
	var mu sync.Mutex
	logCallback := func(line string) {
		mu.Lock()
		streamedLogs = append(streamedLogs, line)
		mu.Unlock()
	}

	// StreamLogs should be called *after* Start, as it depends on task.
	// The current design of processLogs in cont.go starts with task creation.
	// Let's add the callback before start to catch all logs.
	// This implies c.addCallback should be public or StreamLogs be callable before Start to register callback.
	// Given current structure, let's test StreamLogs after start for existing logs, then new ones.
	// Modify: Add callback to c.callbacks directly for this test for simplicity if addCallback not exported
	c.callbacks = append(c.callbacks, logCallback) // Direct modification for test

	err = c.Start()
	require.NoError(t, err)

	// Wait for task to complete
	statusC, err := c.task.Wait(c.ctx)
	require.NoError(t, err)
	select {
	case <-statusC:
		// fine
	case <-time.After(10 * time.Second):
		t.Fatal("Container did not exit for log streaming test")
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Condition(t, func() bool {
		stdoutFound, stderrFound := false, false
		for _, l := range streamedLogs {
			if strings.Contains(l, "stdout_message_stream") {
				stdoutFound = true
			}
			if strings.Contains(l, "stderr_message_stream") {
				stderrFound = true
			}
		}
		return stdoutFound && stderrFound
	}, "Streamed logs did not contain expected stdout/stderr messages. Got: %v", streamedLogs)
}

func TestContainer_TempDirCleanup(t *testing.T) {
	setupContainerdTest(t)
	containerName := "test-tempdir-" + uuid.NewString()
	cfg := ContainerConfig{
		Image:     testImageAlpine,
		Name:      containerName,
		Namespace: testNamespace,
		Command:   []string{"sleep", "0.1"}, // Short-lived
		Env:       []string{},
		RemoveOptions: RemoveOptions{
			RemoveContainerIfExists: true,
			RemoveSnapshotIfExists:  true,
		},
	}

	c, err := NewContainer(cfg)
	require.NoError(t, err)
	// No defer c.Close() here as we want to test finalizer behavior implicitly or explicit Remove

	tempDir, err := os.MkdirTemp("", "kappa-cont-test-tempdir-*")
	require.NoError(t, err)
	c.RegisterTmpDir(tempDir) // Register it

	err = c.Start()
	require.NoError(t, err)

	statusC, _ := c.task.Wait(c.ctx)
	<-statusC // Wait for completion

	err = c.Remove() // This should trigger cleanup
	require.NoError(t, err)

	_, err = os.Stat(tempDir)
	assert.True(t, os.IsNotExist(err), "Temporary directory should be removed after container removal. Path: %s", tempDir)

	// Test cleanup() directly if it were public. For now, test via Remove().
	// Test finalizer is harder, rely on explicit Remove/Stop with RemoveOnStop.
	err = c.Close() // Should be fine to call Close multiple times or after Remove
	assert.NoError(t, err)
}

func TestContainer_StopOptions(t *testing.T) {
	setupContainerdTest(t)

	t.Run("Stop with RemoveOnStop", func(t *testing.T) {
		containerName := "test-stop-remove-" + uuid.NewString()
		cfg := ContainerConfig{
			Image: testImageAlpine, Name: containerName, Namespace: testNamespace, Command: []string{"sleep", "30"},
			RemoveOptions: RemoveOptions{RemoveContainerIfExists: true, RemoveSnapshotIfExists: true},
			Env:           []string{},
		}
		c, err := NewContainer(cfg)
		require.NoError(t, err)
		defer c.Close() // Ensure client is closed

		err = c.Start()
		require.NoError(t, err)

		stopOpts := StopOptions{Timeout: 1 * time.Second, ForceKill: true, RemoveOnStop: true}
		err = c.Stop(stopOpts)
		require.NoError(t, err)

		client, _ := containerd.New(containerdSocket)
		defer client.Close()
		_, err = client.LoadContainer(namespaces.WithNamespace(context.Background(), testNamespace), containerName)
		assert.Error(t, err, "Container should be removed due to RemoveOnStop:true")
	})

	t.Run("Stop without RemoveOnStop", func(t *testing.T) {
		containerName := "test-stop-no-remove-" + uuid.NewString()
		cfg := ContainerConfig{
			Image: testImageAlpine, Name: containerName, Namespace: testNamespace, Command: []string{"sleep", "30"},
			RemoveOptions: RemoveOptions{RemoveContainerIfExists: true, RemoveSnapshotIfExists: true},
			Env:           []string{},
		}
		c, err := NewContainer(cfg)
		require.NoError(t, err)
		defer c.Close()
		defer func() { // Manual cleanup
			_ = c.Stop(StopOptions{Timeout: 1 * time.Second, ForceKill: true, RemoveOnStop: true})
			_ = c.Remove()
		}()

		err = c.Start()
		require.NoError(t, err)

		stopOpts := StopOptions{Timeout: 1 * time.Second, ForceKill: true, RemoveOnStop: false}
		err = c.Stop(stopOpts)
		require.NoError(t, err)

		client, _ := containerd.New(containerdSocket)
		defer client.Close()
		loadedContainer, err := client.LoadContainer(namespaces.WithNamespace(context.Background(), testNamespace), containerName)
		assert.NoError(t, err, "Container should still exist due to RemoveOnStop:false")
		if loadedContainer != nil {
			// Task should be stopped. Getting status might error if task deleted by stop, or show stopped.
			task, taskErr := loadedContainer.Task(c.ctx, nil)
			if taskErr == nil {
				status, statusErr := task.Status(c.ctx)
				require.NoError(t, statusErr)
				assert.True(t, status.Status == containerd.Stopped || status.Status == containerd.Unknown, // Unknown if deleted
					"Task status should be stopped, got %s", status.Status)
			} else {
				assert.ErrorContains(t, taskErr, "not found") // Task might be deleted
			}
		}
	})
}

// Example of mount test (requires a host path)
// Example of mount test (requires a host path)
func TestContainer_WithMount(t *testing.T) {
	setupContainerdTest(t)

	hostTestFilePath := filepath.Join(t.TempDir(), "host-file.txt")
	err := os.WriteFile(hostTestFilePath, []byte("content from host"), 0644)
	require.NoError(t, err)

	containerMountPath := "/mnt/testfile.txt"
	containerName := "test-mount-" + uuid.NewString()

	cfg := ContainerConfig{
		Image:     testImageAlpine,
		Name:      containerName,
		Namespace: testNamespace,
		// Simplified command with better error visibility and timing
		Command: []string{"sh", "-c", fmt.Sprintf(
			"echo 'Starting container'; sleep 0.5; "+
				"echo 'Checking file exists'; ls -la %s; "+
				"echo 'Reading file content:'; cat %s; "+
				"echo 'File read complete'; sleep 0.5; "+
				"echo 'Container exiting'; exit 0",
			containerMountPath, containerMountPath)},
		Env: []string{},
		Mounts: []specs.Mount{
			{
				Type:        "bind",
				Source:      hostTestFilePath,
				Destination: containerMountPath,
				Options:     []string{"ro", "rbind"}, // Read-only
			},
		},
		RemoveOptions: RemoveOptions{RemoveContainerIfExists: true, RemoveSnapshotIfExists: true},
	}

	c, err := NewContainer(cfg)
	require.NoError(t, err)
	defer c.Close()

	defer func() {
		stopOpts := StopOptions{Timeout: 5 * time.Second, ForceKill: true, RemoveOnStop: true}
		_ = c.Stop(stopOpts)
		_ = c.Remove()
	}()

	err = c.Start()
	require.NoError(t, err)

	statusC, err := c.task.Wait(c.ctx)
	require.NoError(t, err)

	var exitStatus containerd.ExitStatus
	select {
	case exitStatus = <-statusC:
		t.Logf("Container exited with code: %d", exitStatus.ExitCode())
	case <-time.After(15 * time.Second): // Increased timeout
		t.Fatalf("Container with mount did not exit in time")
	}

	// Give logs time to be processed after container exit
	time.Sleep(200 * time.Millisecond)

	logs := c.GetLogs()
	t.Logf("Container logs: %v", logs) // Debug logging

	// Check for successful execution first
	assert.EqualValues(t, 0, exitStatus.ExitCode(), "Container script should exit 0. Logs: %v", logs)

	// Then check for expected content
	foundContent := false
	foundFileCheck := false

	for _, logLine := range logs {
		if strings.Contains(logLine, "content from host") {
			foundContent = true
		}
		if strings.Contains(logLine, "File read complete") || strings.Contains(logLine, "Reading file content") {
			foundFileCheck = true
		}
	}

	assert.True(t, foundFileCheck, "Expected file operation logs not found in logs: %v", logs)
	assert.True(t, foundContent, "Expected content from mounted file not found in logs: %v", logs)
}
