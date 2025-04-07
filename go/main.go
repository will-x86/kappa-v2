package main

import (
	"log"
	"os"
	"time"

	"go.uber.org/zap"
	"kappa-v3/internal/cont"
	"kappa-v3/internal/runtime"
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

	c := make(map[string]string)
	c["package.json"] = `{
    "name": "example-app",
    "version": "1.0.0",
    "dependencies": {
        "express": "^4.18.2",
        "axios": "^1.6.2"
    }
}`

	c["index.js"] = `
    const express = require('express');
    const axios = require('axios');
    const helper = require('./lib/helper');
    
    console.log('Starting main application...');
    console.log(helper.getMessage());
    
    // Show that we can access our dependencies
    console.log('Express version:', express.version);
    console.log('Axios version:', axios.version);
    
    // Some async operation
    setTimeout(() => {
        console.log('Async operation completed');
    }, 2000);
`

	c["lib/helper.js"] = `
    module.exports = {
        getMessage: () => {
            return 'Hello from the helper module!';
        }
    };
`

	r := runtime.Runtime{
		Language: "nodejs",
		Version:  "latest",
		Code:     c,
	}

	container, err := r.NewContainer()
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
			logger.Info("Container log", zap.String("line", line))
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
