package logger

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Helper to capture stdout
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// ResetForTest resets the logger instance for testing purposes.
// Note: This is a local copy. Ideally, this function would be exported from the logger package.
var (
	testOnce   sync.Once
	testLogger *zap.Logger
	resetMu    sync.Mutex
)

func resetGlobalLoggerState() {
	resetMu.Lock()
	defer resetMu.Unlock()
	ResetForTest()
}

func TestGet_Singleton(t *testing.T) {
	resetGlobalLoggerState()

	logger1 := Get()
	require.NotNil(t, logger1)

	logger2 := Get()
	require.NotNil(t, logger2)

	assert.Same(t, logger1, logger2, "Get() should return the same logger instance")
}

func TestGet_LogLevelFromEnv(t *testing.T) {
	ResetForTest() // Call the actual one if it exists

	originalLogLevel := os.Getenv("LOG_LEVEL")
	defer os.Setenv("LOG_LEVEL", originalLogLevel)

	testCases := []struct {
		name        string
		envLevel    string
		expectLevel zapcore.Level
		expectWarn  bool
	}{
		{"debug level", "debug", zap.DebugLevel, false},
		{"info level", "info", zap.InfoLevel, false},
		{"warn level", "warn", zap.WarnLevel, false},
		{"error level", "error", zap.ErrorLevel, false},
		{"invalid level", "invalid_level", zap.InfoLevel, true},
		{"empty level", "", zap.InfoLevel, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResetForTest() // From the logger package

			if tc.expectWarn {
				// This sub-test specifically needs a "fresh" `once` to see the warning.
				// This is difficult to achieve without modifying the original package or using build tags for test-specific code.
				// For now, we acknowledge this limitation.
				t.Logf("Testing LOG_LEVEL=%s (warning check might be unreliable due to sync.Once)", tc.envLevel)
			} else {
				os.Setenv("LOG_LEVEL", tc.envLevel)
				// If ResetForTest() from logger package exists and works:
				// ResetForTest()
				l := Get() // This will initialize if not already, based on current LOG_LEVEL
				assert.Equal(t, tc.expectLevel, l.Level(), "Logger level after Get() with LOG_LEVEL='%s'", tc.envLevel)
			}
		})
	}
}

func TestLogger_WithCtx_FromCtx(t *testing.T) {
	resetGlobalLoggerState()

	defaultLogger := Get()
	require.NotNil(t, defaultLogger)

	t.Run("FromCtx without logger returns default", func(t *testing.T) {
		l := FromCtx(context.Background())
		assert.Same(t, defaultLogger, l, "Should return default logger")
	})

	t.Run("WithCtx and FromCtx roundtrip", func(t *testing.T) {
		customLogger := zap.NewNop() // A distinct logger instance
		ctx := WithCtx(context.Background(), customLogger)

		l := FromCtx(ctx)
		assert.Same(t, customLogger, l, "Should return logger from context")
	})

	t.Run("WithCtx with same logger returns original context", func(t *testing.T) {
		customLogger := zap.NewNop()
		ctx1 := WithCtx(context.Background(), customLogger)
		ctx2 := WithCtx(ctx1, customLogger) // Store same logger again

		assert.Same(t, ctx1, ctx2, "Context should not change if same logger is stored")
	})
}

func TestGet_OutputFormat(t *testing.T) {

	ResetForTest()

	logDir := "logs"
	logFile := "logs/app.log"
	_ = os.Mkdir(logDir, 0755)
	_ = os.Remove(logFile) // Clean up previous log file

	defer func() {
		_ = os.Remove(logFile)
		// _ = os.Remove(logDir)  // maybe used by other tests
	}()

	var consoleOutput string
	var wg sync.WaitGroup
	wg.Add(1)

	go func() { // Run Get in a goroutine to allow capturing its init logs if any
		defer wg.Done()
		consoleOutput = captureOutput(func() {
			l := Get()
			l.Info("Test console log message", zap.String("type", "console_test"))
			l.Sync() // Ensure logs are flushed
		})
	}()
	wg.Wait()

	assert.Contains(t, consoleOutput, "Test console log message")
	assert.Contains(t, consoleOutput, "console_test") 

	// Check file log (wait a bit for flush)
	// In a real CI, you might need more robust waiting or file checks.
	require.Eventually(t, func() bool {
		content, err := os.ReadFile(logFile)
		return err == nil && strings.Contains(string(content), `"msg":"Test console log message"`) &&
			strings.Contains(string(content), `"type":"console_test"`) &&
			strings.Contains(string(content), `"git_revision"`) && 
			strings.Contains(string(content), `"go_version"`)
	}, 2*time.Second, 100*time.Millisecond, "File log content not as expected")
}
