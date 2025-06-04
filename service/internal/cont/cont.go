package cont

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"kappa-v2/pkg/logger"
	"os"
	"runtime"
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
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.uber.org/zap"
)

type StopOptions struct {
	Timeout      time.Duration
	ForceKill    bool
	RemoveOnStop bool
}

// sh -c must be done by user
type ContainerConfig struct {
	Image         string   `validate:"required"`
	Name          string   `validate:"required"`
	Namespace     string   `validate:"required"`
	Command       []string `validate:"required"`
	Env           []string `validate:"required"`
	Mounts        []specs.Mount
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
	mounts     []specs.Mount
	client     *containerd.Client
	container  containerd.Container
	task       containerd.Task
	config     ContainerConfig
	ctx        context.Context
	logs       []string
	logMu      sync.Mutex
	callbacks  []LogCallback
	callbackMu sync.Mutex
	tempDirs   []string
	cleanupMu  sync.Mutex
}

func (c *Container) RegisterTmpDir(path string) {
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()
	c.tempDirs = append(c.tempDirs, path)
}

func (c *Container) cleanup() error {
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()

	l := logger.Get()
	var errs []error
	l.Debug("Temp dirs", zap.Any("dirs", c.tempDirs))
	// Clean up temporary directories
	for _, dir := range c.tempDirs {
		l.Info("Removing temporary directory", zap.String("path", dir))
		if err := os.RemoveAll(dir); err != nil {
			l.Error("Failed to remove temporary directory",
				zap.String("path", dir),
				zap.Error(err))
			errs = append(errs, fmt.Errorf("failed to remove temp dir %s: %w", dir, err))
		}
	}
	c.tempDirs = nil

	return errors.Join(errs...)
}

func (c *Container) addCallback(callback LogCallback) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.callbacks = append(c.callbacks, callback)
}

func (c *Container) Task() containerd.Task {
	return c.task
}

func (c *Container) Ctx() context.Context {
	return c.ctx
}

func NewContainer(config ContainerConfig) (*Container, error) {
	l := logger.Get()
	l.Info("Creating new container",
		zap.String("image", config.Image),
		zap.String("name", config.Name),
		zap.String("namespace", config.Namespace))

	// Nice validation :)
	validate := validator.New(validator.WithRequiredStructEnabled())
	if config.Namespace == "" {
		l.Info("Setting default namespace")
		config.Namespace = "default"
	}

	if err := validate.Struct(config); err != nil {
		l.Error("Config validation failed", zap.Error(err))
		return nil, err
	}

	l.Info("Connecting to containerd")
	// TODO: Find out if I should only create 1 of these
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		l.Error("Failed to connect to containerd", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	ctx := namespaces.WithNamespace(context.Background(), config.Namespace)
	l.Info("Container instance created successfully")

	container := &Container{
		id:       config.Name,
		client:   client,
		config:   config,
		ctx:      ctx,
		mounts:   config.Mounts,
		tempDirs: make([]string, 0),
	}
	container.SetupFinalizer()
	return container, nil
}

func (c *Container) Start() error {
	l := logger.Get()
	l.Info("Starting container",
		zap.String("id", c.id),
		zap.String("image", c.config.Image))

	// If it exists should I kill it, this is based on container-name and snapshotter ID, in theory won't be needed in prod as unique file systems etc
	if c.config.RemoveOptions.RemoveContainerIfExists {
		l.Info("Checking for existing container", zap.String("id", c.id))
		if existing, err := c.client.LoadContainer(c.ctx, c.id); err == nil {
			l.Warn("Found existing container, removing it", zap.String("id", c.id))
			if task, err := existing.Task(c.ctx, nil); err == nil {
				l.Info("Found existing task")

				status, err := task.Status(c.ctx)
				if err != nil {
					l.Error("Failed to get task status", zap.Error(err))
					return fmt.Errorf("failed to get task status: %w", err)
				}

				if status.Status == containerd.Running {
					l.Info("Killing existing task")
					if err := task.Kill(c.ctx, syscall.SIGTERM); err != nil {
						l.Warn("SIGTERM failed, trying SIGKILL", zap.Error(err))
						if err := task.Kill(c.ctx, syscall.SIGKILL); err != nil {
							l.Error("Failed to kill task", zap.Error(err))
							return fmt.Errorf("failed to kill task: %w", err)
						}
					}

					statusC, err := task.Wait(c.ctx)
					if err != nil {
						l.Error("Failed to wait for task", zap.Error(err))
						return fmt.Errorf("failed to wait for task: %w", err)
					}

					select {
					case <-statusC:
						l.Info("Task exited")
					case <-time.After(1000 * time.Second):
						l.Warn("Task wait timed out")
					}
				}

				if _, err := task.Delete(c.ctx, containerd.WithProcessKill); err != nil {
					l.Error("Failed to delete task", zap.Error(err))
					return fmt.Errorf("failed to delete task: %w", err)
				}
			}

			if err := existing.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil {
				l.Error("Failed to delete container", zap.Error(err))
				return fmt.Errorf("failed to delete existing container: %w", err)
			}
		}
	}

	if c.config.RemoveOptions.RemoveSnapshotIfExists {
		// Love me some overlayfs
		snapshotter := c.client.SnapshotService("overlayfs")
		snapshotKey := fmt.Sprintf("%s-snapshot", c.id)

		if _, err := snapshotter.Stat(c.ctx, snapshotKey); err == nil {
			l.Warn("Found existing snapshot, removing it", zap.String("snapshot", snapshotKey))
			if err := snapshotter.Remove(c.ctx, snapshotKey); err != nil {
				l.Error("Failed to remove snapshot", zap.Error(err))
				return fmt.Errorf("failed to remove snapshot: %w", err)
			}
		}
	}
	// If exists
	image, err := c.client.GetImage(c.ctx, c.config.Image)
	if err == nil {
		l.Debug("Image already exists, skipping pull")
		// Skip
		goto image_exists
	}
	l.Info("Pulling image")
	image, err = c.client.Pull(c.ctx, c.config.Image, containerd.WithPullUnpack)
	if err != nil {
		l.Error("Failed to pull image", zap.Error(err))
		return fmt.Errorf("failed to pull image: %w", err)
	}
	l.Info("Image pulled successfully")
image_exists:

	for k, v := range c.mounts {
		l.Debug("Mount:", zap.Int("id", k), zap.Any("mount", v))
	}
	l.Info("Creating new container instance")
	container, err := c.client.NewContainer(
		c.ctx,
		c.id,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(c.id+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithMemoryLimit(2000000*8),
			oci.WithCPUs("1"),
			oci.WithImageConfig(image),
			oci.WithEnv(c.config.Env),
			oci.WithProcessArgs(c.config.Command...),
			oci.WithMounts(c.mounts),
			oci.WithProcessCwd("/app"),
			oci.WithHostHostsFile,
			oci.WithHostResolvconf,
			oci.WithHostNamespace(specs.NetworkNamespace),
		),
	)
	if err != nil {
		l.Error("Failed to create container", zap.Error(err))
		return fmt.Errorf("failed to create container: %w", err)
	}

	c.container = container
	l.Info("Creating new task")
	// Pipes for stdi/o used in process logs
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	task, err := container.NewTask(c.ctx, cio.NewCreator(
		cio.WithStreams(nil, stdoutW, stderrW),
	))
	if err != nil {
		l.Error("Failed to create task", zap.Error(err))
		return fmt.Errorf("failed to create task: %w", err)
	}
	go c.processLogs(stderrR, "stderr")
	go c.processLogs(stdoutR, "stdout")
	c.task = task

	l.Info("Starting task")
	if err := task.Start(c.ctx); err != nil {
		l.Error("Failed to start task", zap.Error(err))
		return fmt.Errorf("failed to start task: %w", err)
	}

	l.Info("Container started successfully",
		zap.String("id", c.id),
		zap.String("state", "running"))
	return nil
}

func (c *Container) SetupFinalizer() {
	runtime.SetFinalizer(c, func(c *Container) {
		if err := c.cleanup(); err != nil {
			zap.L().Error("Failed to cleanup in finalizer", zap.Error(err))
		}
	})
}

func (c *Container) Stop(opts StopOptions) error {
	l := logger.Get()
	l.Info("Stopping container", zap.Any("StopOptions", opts))

	if c.task == nil {
		l.Error("No running task found")
		return fmt.Errorf("no running task found")
	}

	/*
	status, err := c.task.Status(c.ctx)
	if err != nil {
		l.Error("Failed to get task status", zap.Error(err))
		return fmt.Errorf("failed to get task status: %w", err)
	}*/

	status, err := c.task.Status(c.ctx)
	if err != nil {
		if !errors.Is(err, errdefs.ErrNotFound){
			l.Warn("Task status check failed", zap.Error(err))
		}
	}

	if status.Status != containerd.Running {
		l.Info("Task is not running, proceeding to cleanup")
		if opts.RemoveOnStop {
			return c.Remove()
		}
		return nil
	}

	signal := syscall.SIGTERM
	if opts.ForceKill {
		signal = syscall.SIGKILL
	}

	l.Info("Sending signal to container", zap.String("signal", signal.String()))
	if err = c.task.Kill(c.ctx, signal); err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			l.Info("Process already finished")
			if opts.RemoveOnStop {
				return c.Remove()
			}
			return nil
		}
		l.Error("Failed to stop container", zap.Error(err))
		return fmt.Errorf("failed to stop container: %w", err)
	}

	l.Info("Waiting for container to stop")
	statusC, err := c.task.Wait(c.ctx)
	if err != nil {
		l.Error("Failed to wait for container", zap.Error(err))
		return fmt.Errorf("failed to wait for container: %w", err)
	}

	select {
	case status := <-statusC:
		l.Info("Container stopped", zap.Uint32("exitCode", status.ExitCode()))
	case <-time.After(opts.Timeout):
		l.Warn("Container stop timed out, forcing kill")
		if err := c.task.Kill(c.ctx, syscall.SIGKILL); err != nil {
			if !errors.Is(err, errdefs.ErrNotFound) {
				l.Error("Failed to force kill container", zap.Error(err))
				return fmt.Errorf("failed to force kill container: %w", err)
			}
		}
	}

	if opts.RemoveOnStop {
		l.Info("Removing container")
		return c.Remove()
	} else {
	}

	return nil
}

// Improved Remove method with better error handling
func (c *Container) Remove() error {
	l := logger.Get()
	l.Info("Removing container", zap.String("id", c.id))
	var errs []error

	if c.task != nil {
		l.Info("Deleting task")
		// Check if task exists before trying to delete
		if _, err := c.task.Status(c.ctx); err == nil {
			if _, err := c.task.Delete(c.ctx); err != nil && !errors.Is(err, errdefs.ErrNotFound) {
				l.Error("Failed to delete task", zap.Error(err))
				errs = append(errs, fmt.Errorf("failed to delete task: %w", err))
			}
		} else if !errors.Is(err, errdefs.ErrNotFound) {
			l.Warn("Task status check failed", zap.Error(err))
		}
	}

	if c.container != nil {
		l.Info("Deleting container")
		if err := c.container.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil && !errors.Is(err, errdefs.ErrNotFound) {
			l.Error("Failed to delete container", zap.Error(err))
			errs = append(errs, fmt.Errorf("failed to delete container: %w", err))
		}
	}

	// Perform cleanup of temporary directories
	if err := c.cleanup(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	l.Info("Container removed successfully")
	return nil
}

// Improved processLogs with better error handling and timing
func (c *Container) processLogs(reader io.Reader, source string) {
	l := logger.Get()
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := fmt.Sprintf("[%s] %s", source, scanner.Text())

		// Store logs
		c.logMu.Lock()
		c.logs = append(c.logs, line)
		c.logMu.Unlock()

		// Call callbacks
		c.callbackMu.Lock()
		callbacks := make([]LogCallback, len(c.callbacks))
		copy(callbacks, c.callbacks)
		c.callbackMu.Unlock()

		for _, cb := range callbacks {
			if cb != nil {
				cb(line)
			}
		}

		l.Debug("Processed log line", zap.String("source", source), zap.String("line", line))
	}

	if err := scanner.Err(); err != nil {
		l.Error("Error scanning logs", zap.String("source", source), zap.Error(err))
	}

	l.Debug("Log processing completed", zap.String("source", source))
}

func (c *Container) WaitForLogs(timeout time.Duration) error {
	if c.task == nil {
		return fmt.Errorf("no task available")
	}

	// Wait for task to complete first
	statusC, err := c.task.Wait(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for task: %w", err)
	}

	select {
	case <-statusC:
		// Task completed, give logs time to be processed
		time.Sleep(100 * time.Millisecond)
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for container to complete")
	}
}

func (c *Container) GetLogs() []string {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	return slices.Clone(c.logs)
}

func (c *Container) Close() error {
	l := logger.Get()
	var errs []error

	c.logMu.Lock()
	c.logs = nil
	c.logMu.Unlock()

	if err := c.cleanup(); err != nil {
		errs = append(errs, err)
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	l.Info("Container closed successfully")
	return nil
}

func (c *Container) StreamLogs(opts LogOptions) error {
	l := logger.Get()
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

	l.Info("Started log streaming")
	return nil
}
