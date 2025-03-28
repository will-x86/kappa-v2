package main

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

func main() {
	log.Println("Starting program")
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("Creating namespace context")
	ctx := namespaces.WithNamespace(context.Background(), "example")

	log.Println("Connecting to containerd")
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return fmt.Errorf("failed to connect to containerd: %w", err)
	}
	defer client.Close()

	log.Println("Getting snapshotter service")
	snapshotter := client.SnapshotService("overlayfs")

	containerID := "alpine-demo"
	snapshotKey := "alpine-demo-snapshot"

	log.Println("Checking for existing container")
	var container containerd.Container
	if container, err = client.LoadContainer(ctx, containerID); err == nil {
		log.Println("Found existing container, cleaning up...")

		log.Println("Checking for existing task")
		var task containerd.Task
		if task, err = container.Task(ctx, nil); err == nil {
			log.Println("Found existing task, killing it")

			if err = task.Kill(ctx, syscall.SIGTERM); err != nil {
				log.Printf("SIGTERM failed: %v, trying SIGKILL", err)
				// If SIGTERM fails, try SIGKILL
				if err = task.Kill(ctx, syscall.SIGKILL); err != nil {
					return fmt.Errorf("failed to kill task with SIGKILL: %w", err)
				}
			}
			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			var statusC <-chan containerd.ExitStatus
			statusC, err = task.Wait(waitCtx)
			if err != nil {
				return fmt.Errorf("failed to wait for task: %w", err)
			}

			select {
			case <-statusC:
				log.Println("Task exited")
			case <-waitCtx.Done():
				log.Println("Task wait timed out, forcing deletion")
			}

			if _, err = task.Delete(ctx, containerd.WithProcessKill); err != nil {
				log.Printf("Failed to delete task: %v", err)
			}
		}
		log.Println("Deleting container")
		if err = container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			return fmt.Errorf("failed to delete existing container: %w", err)
		}
	} else if !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing container: %w", err)
	} else {
		log.Println("No existing container found")
	}

	log.Println("Checking for existing snapshot")
	if _, err = snapshotter.Stat(ctx, snapshotKey); err == nil {
		log.Println("Found existing snapshot, removing it")
		if err = snapshotter.Remove(ctx, snapshotKey); err != nil {
			return fmt.Errorf("failed to remove existing snapshot: %w", err)
		}
	} else {
		log.Println("No existing snapshot found")
	}

	log.Println("Pulling image")
	image, err := client.Pull(ctx, "docker.io/library/alpine:latest", containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	log.Println("Image pulled successfully")

	log.Println("Creating new container")
	container, err = client.NewContainer(
		ctx,
		containerID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(snapshotKey, image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	log.Println("Container created successfully")
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)

	log.Println("Creating new task")
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	log.Println("Task created successfully")
	defer task.Delete(ctx)

	log.Println("Starting task")
	if err = task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	log.Println("Task started successfully")

	log.Println("Setting up wait channel")
	statusC, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for task: %w", err)
	}

	log.Println("Sleeping for 2 seconds")
	time.Sleep(2 * time.Second)

	log.Println("Killing task")
	if err = task.Kill(ctx, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to kill task: %w", err)
	}

	log.Println("Waiting for status")
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return fmt.Errorf("failed to get exit status: %w", err)
	}

	log.Printf("Container exited with code: %d\n", code)
	log.Println("Program completed successfully")
	return nil
}

