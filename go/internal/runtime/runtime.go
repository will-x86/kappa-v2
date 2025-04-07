package runtime

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"kappa-v3/internal/cont"

	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.uber.org/zap"
)

type Runtime struct {
	Language string
	Version  string
	Code     map[string]string
}

type RuntimeConfig struct {
	Image   string
	Command []string
	Mounts  []specs.Mount
	Env     []string
	SetupFn func(string) error
}

func (r Runtime) setupNodeModules() error {
	logger := zap.L()
	logger.Info("Setting up node modules")

	tmpPath, err := os.MkdirTemp("", "kappa-v3-setup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	nodeModulesPath := "/var/kappa-v3/runtimes/nodejs/node_modules"
	if err = os.MkdirAll(nodeModulesPath, 0o777); err != nil {
		os.RemoveAll(tmpPath)
		return fmt.Errorf("failed to create node_modules directory: %w", err)
	}
	if err = os.MkdirAll(tmpPath+"/app", 0o777); err != nil {
		os.RemoveAll(tmpPath)
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	if pkgJSON, ok := r.Code["package.json"]; ok {
		if err = os.WriteFile(filepath.Join(tmpPath, "package.json"), []byte(pkgJSON), 0o644); err != nil {
			os.RemoveAll(tmpPath)
			return fmt.Errorf("failed to write package.json: %w", err)
		}
	} else {
		os.RemoveAll(tmpPath)
		return fmt.Errorf("package.json not found in code")
	}

	logger.Debug("package.json content", zap.String("content", r.Code["package.json"]))

	setupConfig := cont.ContainerConfig{
		Name:    uuid.New().String(),
		Image:   fmt.Sprintf("docker.io/library/node:%s", r.Version),
		Command: []string{"npm", "install", "--verbose"},
		Env:     []string{},
		Mounts: []specs.Mount{
			{
				Type:        "linux",
				Source:      tmpPath,
				Destination: "/app",
				Options:     []string{"rbind", "rw"},
			},
			{
				Type:        "linux",
				Source:      nodeModulesPath,
				Destination: "/app/node_modules",
				Options:     []string{"rbind", "rw"},
			},
		},
		RemoveOptions: cont.RemoveOptions{
			RemoveContainerIfExists: true,
			RemoveSnapshotIfExists:  true,
		},
	}

	logger.Debug("Container setup config", zap.Any("config", setupConfig))

	setupContainer, err := cont.NewContainer(setupConfig)
	if err != nil {
		os.RemoveAll(tmpPath)
		return fmt.Errorf("failed to create setup container: %w", err)
	}

	defer func() {
		setupContainer.Close()
		os.RemoveAll(tmpPath)
	}()

	logger.Info("Starting npm install container")
	logger.Info("Starting npm install container")
	err = setupContainer.Start()
	if err != nil {
		return fmt.Errorf("failed to start setup container: %w", err)
	}

	if err = setupContainer.StreamLogs(cont.LogOptions{
		Follow: true,
		Stdout: true,
		Stderr: true,
		Callback: func(line string) {
			logger.Info("Setup log", zap.String("line", line))
			log.Println(line)
		},
	}); err != nil {
		return fmt.Errorf("failed to stream setup logs: %w", err)
	}

	logger.Info("Waiting for npm install to complete")
	statusC, err := setupContainer.Task().Wait(setupContainer.Ctx())
	if err != nil {
		return fmt.Errorf("failed to wait for container: %w", err)
	}

	select {
	case status := <-statusC:
		logger.Info("npm install completed", zap.Uint32("exitCode", status.ExitCode()))
		if status.ExitCode() != 0 {
			return fmt.Errorf("npm install failed with exit code: %d", status.ExitCode())
		}
	case <-time.After(5 * time.Minute):
		logger.Warn("npm install timed out after 5 minutes, stopping container")
		stopOpts := cont.StopOptions{
			Timeout:      30 * time.Second,
			ForceKill:    true,
			RemoveOnStop: true,
		}
		if err = setupContainer.Stop(stopOpts); err != nil {
			logger.Error("Failed to stop container after timeout", zap.Error(err))
		}
		return fmt.Errorf("npm install timed out after 5 minutes")
	}

	stopOpts := cont.StopOptions{
		Timeout:      30 * time.Second,
		ForceKill:    false,
		RemoveOnStop: true,
	}
	if err = setupContainer.Stop(stopOpts); err != nil {
		logger.Warn("Warning: Failed to clean up container", zap.Error(err))
	}

	nodeModulesPath = "/var/kappa-v3/runtimes/nodejs/node_modules"
	entries, err := os.ReadDir(nodeModulesPath)
	if err != nil {
		return fmt.Errorf("failed to read node_modules directory: %w", err)
	}

	if len(entries) <= 0 {
		return fmt.Errorf("npm install didn't create any modules")
	}

	logger.Info("Node modules setup completed successfully")
	return nil
}

var languageConfigs = map[string]func(version, tmpPath string) RuntimeConfig{
	"nodejs": func(version, tmpPath string) RuntimeConfig {
		return RuntimeConfig{
			Image:   fmt.Sprintf("docker.io/library/node:%s", version),
			Command: []string{"node", "index.js"},
			Mounts: []specs.Mount{
				{
					Type:        "linux",
					Source:      tmpPath,
					Destination: "/app",
					Options:     []string{"rbind", "rw"},
				},
				{
					Type:        "linux",
					Source:      "/var/kappa-v3/runtimes/nodejs/node_modules",
					Destination: "/app/node_modules",
					Options:     []string{"rbind", "ro"},
				},
			},

			Env: []string{},
			SetupFn: func(_ string) error {
				return os.MkdirAll("/var/kappa-v3/runtimes/nodejs/node_modules", os.ModePerm)
			},
		}
	},
	"golang": func(version, tmpPath string) RuntimeConfig {
		return RuntimeConfig{
			Image:   fmt.Sprintf("docker.io/library/golang:%s", version),
			Command: []string{"go", "run", "/app/main.go"},
			Mounts: []specs.Mount{
				{
					Type:        "linux",
					Source:      "/var/kappa-v3/runtimes/golang/pkg",
					Destination: "/go/pkg",
					Options:     []string{"rbind", "ro"},
				},
			},
			SetupFn: func(_ string) error {
				return os.MkdirAll("/var/kappa-v3/runtimes/golang/pkg", os.ModePerm)
			},
		}
	},
}

func (r Runtime) NewContainer() (*cont.Container, error) {
	if r.Language == "nodejs" {
		if err := r.setupNodeModules(); err != nil {
			return nil, fmt.Errorf("failed to setup node moduels %w", err)
		}
	}
	configFn, ok := languageConfigs[r.Language]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", r.Language)
	}

	tmpPath, err := os.MkdirTemp("", "kappa-v3-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	runtimeConfig := configFn(r.Version, tmpPath)

	if runtimeConfig.SetupFn != nil {
		if err = runtimeConfig.SetupFn(tmpPath); err != nil {
			os.RemoveAll(tmpPath)
			return nil, err
		}
	}

	if err = r.createCodeDirs(tmpPath); err != nil {
		os.RemoveAll(tmpPath)
		return nil, err
	}

	config := cont.ContainerConfig{
		Name:    uuid.New().String(),
		Image:   runtimeConfig.Image,
		Command: runtimeConfig.Command,
		Mounts:  runtimeConfig.Mounts,
		Env:     runtimeConfig.Env,
		RemoveOptions: cont.RemoveOptions{
			RemoveContainerIfExists: true,
			RemoveSnapshotIfExists:  true,
		},
	}
	container, err := cont.NewContainer(config)
	if err != nil {
		os.RemoveAll(tmpPath)
		return nil, err
	}
	container.RegisterTmpDir(tmpPath)
	return container, nil
}

func (r Runtime) createCodeDirs(tmpPath string) error {
	if err := os.MkdirAll(tmpPath, 0o755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	for path, code := range r.Code {
		fullPath := filepath.Join(tmpPath, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(code), 0o644); err != nil {
			return fmt.Errorf("failed to write code to %s: %w", fullPath, err)
		}
	}
	return nil
}
