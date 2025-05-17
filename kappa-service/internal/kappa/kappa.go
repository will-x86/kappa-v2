package kappa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kappa-service/internal/cont"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.uber.org/zap"
)

// KappaEvent represents the data sent to the kappa function.
type KappaEvent struct {
	Body        map[string]any    `json:"body"`
	Path        string            `json:"path"`
	HTTPMethod  string            `json:"httpMethod"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"queryParams"`
	RequestID   string            `json:"requestId"`
}

// KappaResponse represents the response from the kappa function.
type KappaResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       map[string]any    `json:"body"`
	RequestID  string            `json:"requestId"`
}

// KappaFunction represents a containerized kappa function.
type KappaFunction struct {
	Name              string
	BinaryPath        string
	Image             string
	Env               []string
	Port              int
	container         *cont.Container
	containerURL      string
	runtimeAPIPort    int
	logs              []string
	logsMu            sync.Mutex
	isRunning         bool
	isRunningMu       sync.Mutex
	requestsProcessed int
	idleTimeout       time.Duration
	idleTimer         *time.Timer
	idleTimerMu       sync.Mutex
}

// NewKappaFunction creates a new kappa function instance.
func NewKappaFunction(name, binaryPath, image string, env []string, port int) *KappaFunction {
	return &KappaFunction{
		Name:        name,
		BinaryPath:  binaryPath,
		Image:       image,
		Env:         env,
		Port:        port,
		isRunning:   false,
		idleTimeout: 5 * time.Minute, // Default idle timeout: 5 minutes
	}
}

// SetIdleTimeout sets the idle timeout after which the container will be stopped.
func (lf *KappaFunction) SetIdleTimeout(duration time.Duration) {
	lf.idleTimerMu.Lock()
	defer lf.idleTimerMu.Unlock()

	lf.idleTimeout = duration
	if lf.idleTimer != nil {
		lf.idleTimer.Reset(duration)
	}
}

// Start starts the kappa function container.
func (lf *KappaFunction) Start(ctx context.Context) error {
	lf.isRunningMu.Lock()
	defer lf.isRunningMu.Unlock()

	if lf.isRunning {
		return nil // Already running
	}

	logger := zap.L()
	logger.Info("Starting kappa function",
		zap.String("name", lf.Name),
		zap.String("binary", lf.BinaryPath))

	// Create temp directory for the binary
	tmpPath, err := os.MkdirTemp("", fmt.Sprintf("kappa-kappa-%s-*", lf.Name))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Copy the binary to the temp directory
	destBinary := filepath.Join(tmpPath, "main")
	if err := os.Link(lf.BinaryPath, destBinary); err != nil {
		if err := copyFile(lf.BinaryPath, destBinary); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
	}

	// Make binary executable
	if err := os.Chmod(destBinary, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Base environment variables
	env := append([]string{
		fmt.Sprintf("PORT=%d", lf.Port),
		"LAMBDA_TASK_ROOT=/app",
		fmt.Sprintf("LAMBDA_FUNCTION_NAME=%s", lf.Name),
		"AWS_LAMBDA_RUNTIME_API=localhost:8080", // This will be used by AWS Kappa SDK
	}, lf.Env...)

	// Create container
	container, err := cont.NewContainer(cont.ContainerConfig{
		Image:     lf.Image,
		Name:      fmt.Sprintf("kappa-%s-%s", lf.Name, uuid.New().String()),
		Command:   []string{"/app/main"},
		Env:       env,
		Namespace: "kappa",
		Mounts: []specs.Mount{
			{
				Type:        "bind",
				Source:      tmpPath,
				Destination: "/app",
				Options:     []string{"rbind", "rw"},
			},
		},
		RemoveOptions: cont.RemoveOptions{
			RemoveSnapshotIfExists:  true,
			RemoveContainerIfExists: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	container.RegisterTmpDir(tmpPath)

	// Start container
	if err = container.Start(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Stream logs
	err = container.StreamLogs(cont.LogOptions{
		Follow: true,
		Stdout: true,
		Stderr: true,
		Callback: func(line string) {
			lf.logsMu.Lock()
			lf.logs = append(lf.logs, line)
			if len(lf.logs) > 1000 {
				// Keep log buffer manageable
				lf.logs = lf.logs[len(lf.logs)-1000:]
			}
			lf.logsMu.Unlock()
			logger.Debug("Kappa log", zap.String("function", lf.Name), zap.String("log", line))
		},
	})
	if err != nil {
		return fmt.Errorf("failed to stream logs: %w", err)
	}

	lf.container = container
	lf.containerURL = fmt.Sprintf("http://localhost:%d", lf.Port)
	lf.isRunning = true

	// Start idle timer
	lf.resetIdleTimer()

	logger.Info("Kappa function started",
		zap.String("name", lf.Name),
		zap.String("url", lf.containerURL))

	return nil
}

// Stop stops the kappa function container.
func (lf *KappaFunction) Stop() error {
	lf.isRunningMu.Lock()
	defer lf.isRunningMu.Unlock()

	if !lf.isRunning || lf.container == nil {
		return nil // Already stopped
	}

	stopOpts := cont.StopOptions{
		Timeout:      10 * time.Second,
		ForceKill:    true,
		RemoveOnStop: true,
	}

	lf.cancelIdleTimer()

	err := lf.container.Stop(stopOpts)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	lf.isRunning = false
	zap.L().Info("Kappa function stopped", zap.String("name", lf.Name))
	return nil
}

// resetIdleTimer resets the idle timer.
func (lf *KappaFunction) resetIdleTimer() {
	lf.idleTimerMu.Lock()
	defer lf.idleTimerMu.Unlock()

	if lf.idleTimer != nil {
		lf.idleTimer.Stop()
	}

	lf.idleTimer = time.AfterFunc(lf.idleTimeout, func() {
		// Only stop if it's still running when the timer fires
		lf.isRunningMu.Lock()
		isRunning := lf.isRunning
		lf.isRunningMu.Unlock()

		if isRunning {
			zap.L().Info("Stopping idle kappa function", zap.String("name", lf.Name))
			_ = lf.Stop()
		}
	})
}

// cancelIdleTimer cancels the idle timer.
func (lf *KappaFunction) cancelIdleTimer() {
	lf.idleTimerMu.Lock()
	defer lf.idleTimerMu.Unlock()

	if lf.idleTimer != nil {
		lf.idleTimer.Stop()
		lf.idleTimer = nil
	}
}

// Invoke invokes the kappa function with the given event.
func (lf *KappaFunction) Invoke(ctx context.Context, event KappaEvent) (*KappaResponse, error) {
	// First ensure the function is running
	lf.isRunningMu.Lock()
	isRunning := lf.isRunning
	lf.isRunningMu.Unlock()

	if !isRunning {
		if err := lf.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start kappa function: %w", err)
		}

		// Give the container a moment to initialize
		// time.Sleep(500 * time.Millisecond)
	}

	// Reset the idle timer since we're about to make a request
	lf.resetIdleTimer()

	// Generate a request ID if not already present
	if event.RequestID == "" {
		event.RequestID = uuid.New().String()
	}

	// Prepare the request
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Make the HTTP request to the container
	url := fmt.Sprintf("%s/2015-03-31/functions/function/invocations", lf.containerURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Kappa-Runtime-Aws-Request-Id", event.RequestID)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		// If we get a connection error, maybe the container is not ready yet
		// Try to restart it once
		if lf.isRunning {
			zap.L().Warn("Failed to connect to kappa function, attempting to restart",
				zap.String("name", lf.Name),
				zap.Error(err))

			// Stop and restart
			_ = lf.Stop()
			if err := lf.Start(ctx); err != nil {
				return nil, fmt.Errorf("failed to restart kappa function: %w", err)
			}

			// Wait for startup
			time.Sleep(1 * time.Second)

			// Try again
			resp, err = client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("failed to invoke kappa function after restart: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to invoke kappa function: %w", err)
		}
	}
	defer resp.Body.Close()

	// Parse the response
	var kappaResp KappaResponse
	if err := json.NewDecoder(resp.Body).Decode(&kappaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Set the request ID if not set in the response
	if kappaResp.RequestID == "" {
		kappaResp.RequestID = event.RequestID
	}

	// Increment requests processed
	lf.requestsProcessed++

	return &kappaResp, nil
}

// GetLogs returns the logs from the container.
func (lf *KappaFunction) GetLogs() []string {
	lf.logsMu.Lock()
	defer lf.logsMu.Unlock()

	logs := make([]string, len(lf.logs))
	copy(logs, lf.logs)
	return logs
}

// IsRunning returns true if the kappa function is running.
func (lf *KappaFunction) IsRunning() bool {
	lf.isRunningMu.Lock()
	defer lf.isRunningMu.Unlock()
	return lf.isRunning
}

// Utility function to copy files when hard linking fails
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
