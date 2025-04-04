package cont

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type StopOptions struct {
	Timeout      time.Duration
	ForceKill    bool
	RemoveOnStop bool
}

// sh -c must be done by user
type ContainerConfig struct {
	Image         string `validate:"required"`
	Name          string
	Namespace     string   `validate:"required"`
	Command       []string `validate:"required"`
	Env           []string `validate:"required"`
	WorkingDir    string
	RemoveOptions RemoveOptions
}
type RemoveOptions struct {
	RemoveSnapshotIfExists  bool
	RemoveContainerIfExists bool
}

type (
	LogCallback func(line string)
	LogOptions  struct {
		Follow   bool
		Stdout   bool
		Stderr   bool
		Callback LogCallback
	}
)

type Container struct {
	id         string
	client     *containerd.Client
	container  containerd.Container
	task       containerd.Task
	config     ContainerConfig
	ctx        context.Context
	logs       []string
	logMu      sync.Mutex
	callbacks  []LogCallback
	callbackMu sync.Mutex
}

func (c *Container) addCallback(callback LogCallback) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.callbacks = append(c.callbacks, callback)
}

func NewContainer(config ContainerConfig) (*Container, error) {
	logger := zap.L()
	logger.Info("Creating new container",
		zap.String("image", config.Image),
		zap.String("name", config.Name),
		zap.String("namespace", config.Namespace))

	// Nice validation :)
	validate := validator.New(validator.WithRequiredStructEnabled())
	if config.Namespace == "" {
		logger.Info("Setting default namespace")
		config.Namespace = "default"
	}

	if err := validate.Struct(config); err != nil {
		logger.Error("Config validation failed", zap.Error(err))
		return nil, err
	}

	logger.Info("Connecting to containerd")
	// TODO: Find out if I should only create 1 of these
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		logger.Error("Failed to connect to containerd", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	ctx := namespaces.WithNamespace(context.Background(), config.Namespace)
	logger.Info("Container instance created successfully")

	return &Container{
		id:     config.Name,
		client: client,
		config: config,
		ctx:    ctx,
	}, nil
}

func (c *Container) Start() error {
	logger := zap.L()
	logger.Info("Starting container",
		zap.String("id", c.id),
		zap.String("image", c.config.Image))

	// If it exists should I kill it, this is based on container-name and snapshotter ID, in theory won't be needed in prod as unique file systems etc
	if c.config.RemoveOptions.RemoveContainerIfExists {
		logger.Info("Checking for existing container", zap.String("id", c.id))
		if existing, err := c.client.LoadContainer(c.ctx, c.id); err == nil {
			logger.Warn("Found existing container, removing it", zap.String("id", c.id))
			if task, err := existing.Task(c.ctx, nil); err == nil {
				logger.Info("Found existing task")

				status, err := task.Status(c.ctx)
				if err != nil {
					logger.Error("Failed to get task status", zap.Error(err))
					return fmt.Errorf("failed to get task status: %w", err)
				}

				if status.Status == containerd.Running {
					logger.Info("Killing existing task")
					if err := task.Kill(c.ctx, syscall.SIGTERM); err != nil {
						logger.Warn("SIGTERM failed, trying SIGKILL", zap.Error(err))
						if err := task.Kill(c.ctx, syscall.SIGKILL); err != nil {
							logger.Error("Failed to kill task", zap.Error(err))
							return fmt.Errorf("failed to kill task: %w", err)
						}
					}

					statusC, err := task.Wait(c.ctx)
					if err != nil {
						logger.Error("Failed to wait for task", zap.Error(err))
						return fmt.Errorf("failed to wait for task: %w", err)
					}

					select {
					case <-statusC:
						logger.Info("Task exited")
					case <-time.After(5 * time.Second):
						logger.Warn("Task wait timed out")
					}
				}

				if _, err := task.Delete(c.ctx, containerd.WithProcessKill); err != nil {
					logger.Error("Failed to delete task", zap.Error(err))
					return fmt.Errorf("failed to delete task: %w", err)
				}
			}

			if err := existing.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil {
				logger.Error("Failed to delete container", zap.Error(err))
				return fmt.Errorf("failed to delete existing container: %w", err)
			}
		}
	}

	if c.config.RemoveOptions.RemoveSnapshotIfExists {
		// Love me some overlayfs
		snapshotter := c.client.SnapshotService("overlayfs")
		snapshotKey := fmt.Sprintf("%s-snapshot", c.id)

		if _, err := snapshotter.Stat(c.ctx, snapshotKey); err == nil {
			logger.Warn("Found existing snapshot, removing it", zap.String("snapshot", snapshotKey))
			if err := snapshotter.Remove(c.ctx, snapshotKey); err != nil {
				logger.Error("Failed to remove snapshot", zap.Error(err))
				return fmt.Errorf("failed to remove snapshot: %w", err)
			}
		}
	}

	logger.Info("Pulling image")
	image, err := c.client.Pull(c.ctx, c.config.Image, containerd.WithPullUnpack)
	if err != nil {
		logger.Error("Failed to pull image", zap.Error(err))
		return fmt.Errorf("failed to pull image: %w", err)
	}
	logger.Info("Image pulled successfully")

	logger.Info("Creating new container instance")
	container, err := c.client.NewContainer(
		c.ctx,
		c.id,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(c.id+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithEnv(c.config.Env),
			oci.WithProcessArgs(c.config.Command...),
		),
	)
	if err != nil {
		logger.Error("Failed to create container", zap.Error(err))
		return fmt.Errorf("failed to create container: %w", err)
	}

	c.container = container
	logger.Info("Creating new task")
	// Pipes for stdi/o used in process logs
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	task, err := container.NewTask(c.ctx, cio.NewCreator(
		cio.WithStreams(nil, stdoutW, stderrW),
	))
	if err != nil {
		logger.Error("Failed to create task", zap.Error(err))
		return fmt.Errorf("failed to create task: %w", err)
	}
	go c.processLogs(stderrR, "stderr")
	go c.processLogs(stdoutR, "stdout")
	c.task = task

	logger.Info("Starting task")
	if err := task.Start(c.ctx); err != nil {
		logger.Error("Failed to start task", zap.Error(err))
		return fmt.Errorf("failed to start task: %w", err)
	}

	logger.Info("Container started successfully",
		zap.String("id", c.id),
		zap.String("state", "running"))
	return nil
}

func (c *Container) Stop(opts StopOptions) error {
	logger := zap.L()
	logger.Info("Stopping container",
		zap.String("id", c.id),
		zap.Duration("timeout", opts.Timeout),
		zap.Bool("forceKill", opts.ForceKill))

	if c.task == nil {
		logger.Error("No running task found")
		return fmt.Errorf("no running task found")
	}

	status, err := c.task.Status(c.ctx)
	if err != nil {
		logger.Error("Failed to get task status", zap.Error(err))
		return fmt.Errorf("failed to get task status: %w", err)
	}

	if status.Status != containerd.Running {
		logger.Info("Task is not running, proceeding to cleanup")
		if opts.RemoveOnStop {
			return c.Remove()
		}
		return nil
	}

	signal := syscall.SIGTERM
	if opts.ForceKill {
		signal = syscall.SIGKILL
	}

	logger.Info("Sending signal to container", zap.String("signal", signal.String()))
	if err = c.task.Kill(c.ctx, signal); err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			logger.Info("Process already finished")
			if opts.RemoveOnStop {
				return c.Remove()
			}
			return nil
		}
		logger.Error("Failed to stop container", zap.Error(err))
		return fmt.Errorf("failed to stop container: %w", err)
	}

	logger.Info("Waiting for container to stop")
	statusC, err := c.task.Wait(c.ctx)
	if err != nil {
		logger.Error("Failed to wait for container", zap.Error(err))
		return fmt.Errorf("failed to wait for container: %w", err)
	}

	select {
	case status := <-statusC:
		logger.Info("Container stopped", zap.Uint32("exitCode", status.ExitCode()))
	case <-time.After(opts.Timeout):
		logger.Warn("Container stop timed out, forcing kill")
		if err := c.task.Kill(c.ctx, syscall.SIGKILL); err != nil {
			if !errors.Is(err, errdefs.ErrNotFound) {
				logger.Error("Failed to force kill container", zap.Error(err))
				return fmt.Errorf("failed to force kill container: %w", err)
			}
		}
	}

	if opts.RemoveOnStop {
		logger.Info("Removing container")
		return c.Remove()
	}

	return nil
}

func (c *Container) Remove() error {
	logger := zap.L()
	logger.Info("Removing container", zap.String("id", c.id))

	if c.task != nil {
		logger.Info("Deleting task")
		if _, err := c.task.Delete(c.ctx); err != nil {
			logger.Error("Failed to delete task", zap.Error(err))
			return fmt.Errorf("failed to delete task: %w", err)
		}
	}

	if c.container != nil {
		logger.Info("Deleting container")
		if err := c.container.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil {
			logger.Error("Failed to delete container", zap.Error(err))
			return fmt.Errorf("failed to delete container: %w", err)
		}
	}

	logger.Info("Container removed successfully")
	return nil
}

func (c *Container) processLogs(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// Will probably change later to define err and stdout
		line := fmt.Sprintf("[%s] %s", source, scanner.Text())
		c.logMu.Lock()
		c.logs = append(c.logs, line)
		c.logMu.Unlock()

		c.callbackMu.Lock()
		for _, cb := range c.callbacks {
			cb(line)
		}
		c.callbackMu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		zap.L().Error("Error scanning logs",
			zap.String("source", source),
			zap.Error(err))
	}
}

func (c *Container) GetLogs() []string {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	return slices.Clone(c.logs)
}

func (c *Container) Close() error {
	c.logMu.Lock()
	c.logs = nil
	defer c.logMu.Unlock()
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Container) StreamLogs(opts LogOptions) error {
	logger := zap.L()
	if c.task == nil {
		return fmt.Errorf("no running task found")
	}

	if opts.Callback != nil {
		c.logMu.Lock()
		for _, line := range c.logs {
			opts.Callback(line)
		}
		c.logMu.Unlock()

		c.addCallback(opts.Callback)
	}

	logger.Info("Started log streaming")
	return nil
}
