package kappa

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testHandlerBinaryName = "test_handler_main"
	testKappaImage        = "docker.io/library/alpine:latest" 
	containerdSocket      = "/run/containerd/containerd.sock"
)

var (
	// globalTestHandlerBinaryPath is set by TestMain after successful build.
	globalTestHandlerBinaryPath string
	// globalBuildBinaryErr stores any error from the one-time build.
	globalBuildBinaryErr error
)

// buildTestHandlerForSuite is the actual build logic, called once by TestMain.
func buildTestHandlerForSuite() (string, error) {
	handlerSourcePath := "../../handler_example/main.go" // Relative to service/internal/kappa
	if _, err := os.Stat(handlerSourcePath); os.IsNotExist(err) {
		handlerSourcePath = "../../../handler_example/main.go" // If test is run from service/internal/kappa subdir
		if _, err := os.Stat(handlerSourcePath); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			return "", fmt.Errorf("handler_example/main.go not found at common relative paths. CWD: %s. Tried initial '%s' and fallback '%s'", cwd, "../../handler_example/main.go", "../../../handler_example/main.go")
		}
	}

	tempDir, err := os.MkdirTemp("", "kappa-test-suite-bin")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir for suite binary: %w", err)
	}

	outputBinaryPath := filepath.Join(tempDir, testHandlerBinaryName)

	cmd := exec.Command("go", "build", "-o", outputBinaryPath, handlerSourcePath)
	// Build for Linux AMD64 and disable CGO for static linking suitable for Alpine.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	cmd.Stderr = os.Stderr // Show build errors
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir) // Clean up temp dir if build fails
		return "", fmt.Errorf("failed to build test handler: %w", err)
	}
	return outputBinaryPath, nil
}

func TestMain(m *testing.M) {
	// Setup: build the binary once for the entire test suite
	globalTestHandlerBinaryPath, globalBuildBinaryErr = buildTestHandlerForSuite()
	if globalBuildBinaryErr != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to build test handler binary for suite: %v\n", globalBuildBinaryErr)
		// If tempDir might have been created before build failure
		if globalTestHandlerBinaryPath != "" && strings.HasPrefix(filepath.Base(filepath.Dir(globalTestHandlerBinaryPath)), "kappa-test-suite-bin") {
			os.RemoveAll(filepath.Dir(globalTestHandlerBinaryPath))
		}
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: clean up the globally built binary's directory
	if globalTestHandlerBinaryPath != "" {
		os.RemoveAll(filepath.Dir(globalTestHandlerBinaryPath))
	}

	os.Exit(code)
}

// buildTestHandler now returns the pre-built path and error set by TestMain.
func buildTestHandler() (string, error) {
	return globalTestHandlerBinaryPath, globalBuildBinaryErr
}

func setupKappaTest(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(containerdSocket); os.IsNotExist(err) {
		t.Skipf("containerd socket not found at %s, skipping kappa integration test", containerdSocket)
	}

	binaryPath, err := buildTestHandler() // Gets the global path set by TestMain
	// TestMain exits on build error, but this check is a safeguard.
	require.NoError(t, err, "Test handler binary should have been built successfully by TestMain.")
	require.NotEmpty(t, binaryPath, "Test handler binary path (from TestMain) is empty.")

	return binaryPath
}

func TestNewKappaFunction(t *testing.T) {
	fn := NewKappaFunction("testfn", "/path/to/bin", "img", []string{"E=V"}, 8080)
	assert.Equal(t, "testfn", fn.Name)
	assert.Equal(t, "/path/to/bin", fn.BinaryPath)
	assert.Equal(t, "img", fn.Image)
	assert.Equal(t, []string{"E=V"}, fn.Env)
	assert.Equal(t, 8080, fn.Port)
	assert.False(t, fn.IsRunning())
	assert.Equal(t, 5*time.Minute, fn.idleTimeout) // Default
}

func TestKappaFunction_SetIdleTimeout(t *testing.T) {
	fn := NewKappaFunction("testfn", "", "", nil, 0)
	newTimeout := 10 * time.Minute
	fn.SetIdleTimeout(newTimeout)
	assert.Equal(t, newTimeout, fn.idleTimeout)
	// Test reset if timer was active (harder to test without exposing timer state)
}

func TestKappaFunction_StartStop_Lifecycle(t *testing.T) {
	binaryPath := setupKappaTest(t)
	fnName := "lifecycle-" + filepath.Base(t.Name())
	if len(fnName) > 75 { // max len
		fnName = fnName[0:74]
	}
	fn := NewKappaFunction(fnName, binaryPath, testKappaImage, nil, 9091) // Unique port

	// Cleanup function resources
	defer func() {
		if fn.IsRunning() {
			_ = fn.Stop()
		}
		// If container object exists, try to clean it up if stop failed.
		// fn.Stop() should handle removal if RemoveOnStop is true in cont.StopOptions.
		// This is a fallback.
		if fn.container != nil {
			// Check if already stopped to avoid error from removing a running container's resources.
			// However, fn.Stop() handles this. If Stop failed, container might still exist.
			_ = fn.container.Remove()
		}
	}()

	err := fn.Start(context.Background())
	require.NoError(t, err, "fn.Start() failed")
	assert.True(t, fn.IsRunning(), "Function should be running after Start")
	assert.NotNil(t, fn.container, "Container should be initialized")
	assert.NotEmpty(t, fn.containerURL, "Container URL should be set")

	// Check for some log indication of startup
	require.Eventually(t, func() bool {
		logs := fn.GetLogs()
		for _, l := range logs {
			if strings.Contains(l, fmt.Sprintf("Kappa function starting on port %d", fn.Port)) {
				return true
			}
		}
		return false
	}, 10*time.Second, 250*time.Millisecond, "Startup log message not found. Logs: %v", fn.GetLogs()) // Increased timeout and added logs

	err = fn.Stop()
	require.NoError(t, err, "fn.Stop() failed")
	assert.False(t, fn.IsRunning(), "Function should not be running after Stop")
}

func TestKappaFunction_Invoke_Success(t *testing.T) {
	binaryPath := setupKappaTest(t)
	fnName := "invoke-success-" + filepath.Base(t.Name())

	if len(fnName) > 75 {
		fnName = fnName[0:74]
	}
	fn := NewKappaFunction(fnName, binaryPath, testKappaImage, nil, 9092)

	defer func() {
		if fn.IsRunning() {
			_ = fn.Stop()
		}
		if fn.container != nil {
			_ = fn.container.Remove()
		}
	}()

	ctx := context.Background()
	err := fn.Start(ctx)
	require.NoError(t, err, "Failed to start function for invocation")

	// Was an old delay, no longer
	//time.Sleep(1 * time.Second) 

	event := KappaEvent{
		Body: map[string]any{"name": "TestUser"},
	}
	resp, errInvoke := fn.Invoke(ctx, event)
	// Provide more context on Invoke failure
	if errInvoke != nil {
		logs := fn.GetLogs()
		t.Logf("Invoke failed. Function logs: %v", logs)
	}
	require.NoError(t, errInvoke, "fn.Invoke() failed")
	require.NotNil(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, resp.RequestID)

	expectedMessage := "Hello, TestUser! Welcome to your Kappa function!"
	assert.Equal(t, expectedMessage, resp.Body["message"])

	inputBody, ok := resp.Body["input"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "TestUser", inputBody["name"])
}

func TestKappaFunction_Invoke_StartIfNeeded(t *testing.T) {
	binaryPath := setupKappaTest(t)
	fnName := "invoke-autostart-" + filepath.Base(t.Name())
	if len(fnName) > 75 {
		fnName = fnName[0:74]
	}
	fn := NewKappaFunction(fnName, binaryPath, testKappaImage, nil, 9093)
	defer func() {
		if fn.IsRunning() {
			_ = fn.Stop()
		}
		if fn.container != nil {
			_ = fn.container.Remove()
		}
	}()

	assert.False(t, fn.IsRunning(), "Function should not be running initially")

	ctx := context.Background()
	event := KappaEvent{
		Body: map[string]any{"name": "AutoStartUser"},
	}
	resp, errInvoke := fn.Invoke(ctx, event)
	if errInvoke != nil {
		logs := fn.GetLogs()
		t.Logf("Invoke (auto-start) failed. Function logs: %v", logs)
	}
	require.NoError(t, errInvoke, "fn.Invoke() failed on auto-start")
	require.NotNil(t, resp)

	assert.True(t, fn.IsRunning(), "Function should be running after first Invoke")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Body["message"], "AutoStartUser")
}

func TestKappaFunction_IdleTimeout(t *testing.T) {
	binaryPath := setupKappaTest(t)
	fnName := "idle-timeout-" + filepath.Base(t.Name())
	if len(fnName) > 75 {
		fnName = fnName[0:74]
	}
	fn := NewKappaFunction(fnName, binaryPath, testKappaImage, nil, 9094)
	defer func() {
		if fn.IsRunning() {
			_ = fn.Stop()
		}
		if fn.container != nil {
			_ = fn.container.Remove()
		}
	}()

	idleTestTimeout := 2 * time.Second // Short timeout for test, ensure it's longer than typical startup
	fn.SetIdleTimeout(idleTestTimeout)

	ctx := context.Background()
	err := fn.Start(ctx)
	require.NoError(t, err, "Failed to start function for idle test: %v", err)
	assert.True(t, fn.IsRunning(), "Function should be running after start")

	// Wait for longer than idle timeout, ensure this wait also accommodates container startup
	time.Sleep(idleTestTimeout + 1*time.Second)

	assert.False(t, fn.IsRunning(), "Function should be stopped by idle timeout")

	// Try to invoke again, it should restart
	event := KappaEvent{Body: map[string]any{"name": "AfterIdle"}}
	resp, errInvoke := fn.Invoke(ctx, event)
	if errInvoke != nil {
		logs := fn.GetLogs() // Logs from previous run might be gone, these are from new run
		t.Logf("Invoke (after idle) failed. Function logs: %v", logs)
	}
	require.NoError(t, errInvoke, "Invoke after idle stop failed")
	require.NotNil(t, resp)
	assert.True(t, fn.IsRunning(), "Function should restart on invoke after idle stop")
	assert.Contains(t, resp.Body["message"], "AfterIdle")
}

