package main

import (
	"fmt"
	"kappa-v3/internal/cont"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Boring logging stuff
func init() {
	logger := zap.Must(zap.NewProduction())
	if os.Getenv("APP_ENV") == "development" {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger = zap.Must(config.Build())
	}
	zap.ReplaceGlobals(logger)
}

func main() {
	logger := zap.L()
	logger.Info("Starting application")
	tmpPath, err := os.MkdirTemp("", "kappa-v3-setup-*")
	if err != nil {
		panic(err)
	}
	err = os.Link("./code_example/main", fmt.Sprintf("%s/main", tmpPath))
	if err != nil {
		panic(err)
	}

	container, err := cont.NewContainer(cont.ContainerConfig{
		Image:   "docker.io/library/alpine:latest",
		Name:    uuid.New().String(),
		Command: []string{"/app/main"},
		Env:     []string{},
		Mounts: []specs.Mount{
			{
				Type:        "linux",
				Source:      tmpPath,
				Destination: "/app",
				Options:     []string{"rbind", "rw"},
			},
		},
	})
	container.RegisterTmpDir(tmpPath)
	logger.Info("Starting container")
	if err = container.Start(); err != nil {
		logger.Fatal("Failed to start container", zap.Error(err))
	}

	err = container.StreamLogs(cont.LogOptions{
		Follow: true,
		Stdout: true,
		Stderr: true,
		Callback: func(line string) {
			logger.Sugar().Debugln(line)
		},
	})
	if err != nil {
		logger.Fatal("Fatal err with streaming logs", zap.Error(err))
	}

	stopOpts := cont.StopOptions{
		Timeout:      5 * time.Second,
		ForceKill:    true,
		RemoveOnStop: true,
	}

	logger.Info("Stopping container")
	if err := container.Stop(stopOpts); err != nil {
		logger.Fatal("Failed to stop container", zap.Error(err))
	}

	// log.Println("logs", container.GetLogs())
	logger.Info("Application completed successfully")
}
