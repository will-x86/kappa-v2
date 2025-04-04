package main

import (
	"log"
	"os"
	"time"

	"go.uber.org/zap"

	"kappa-v3/internal/cont"
)

// Boring logging stuff
func init() {
	logger := zap.Must(zap.NewProduction())
	if os.Getenv("APP_ENV") == "development" {
		logger = zap.Must(zap.NewDevelopment())
	}

	zap.ReplaceGlobals(logger)
}

func main() {
	logger := zap.L()
	logger.Info("Starting application")

	// Defaults to "default" namespace
	config := cont.ContainerConfig{
		Image:     "docker.io/library/alpine:latest",
		Name:      "my-container",
		Namespace: "example",
		Command:   []string{"sh", "-c", "ls"},
		Env:       []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		RemoveOptions: cont.RemoveOptions{
			RemoveSnapshotIfExists:  true,
			RemoveContainerIfExists: true,
		},
	}

	logger.Info("Creating container", zap.String("name", config.Name))
	container, err := cont.NewContainer(config)
	if err != nil {
		logger.Fatal("Failed to create container", zap.Error(err))
	}
	defer container.Close()

	logger.Info("Starting container")
	if err = container.Start(); err != nil {
		logger.Fatal("Failed to start container", zap.Error(err))
	}

	err = container.StreamLogs(cont.LogOptions{
		Follow: true,
		Stdout: true,
		Stderr: true,
		Callback: func(line string) {
			logger.Info("Inside container logs, ", zap.String("line", line))
		},
	})
	logger.Info("Waiting for 5 seconds")
	time.Sleep(5 * time.Second)
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
	log.Println("logs", container.GetLogs())

	logger.Info("Application completed successfully")
}
