// This package acts as the execution engine on the server side. It manages the runtime lifecycle
// of functions, creating starting, and cleaning up Docker containers to execute the function when
// triggered. It's responsible for running the function in a container and handling input/output.
package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/akos011221/serverless/pkg/storage"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// Orchestrator manages containerized function execution.

type Orchestrator struct {
	docker *client.Client
	log    *logrus.Logger
}

// NewOrchestrator initializes the orchestrator.
func NewOrchestrator(log *logrus.Logger) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}
	return &Orchestrator{docker: cli, log: log}, nil
}

// Execute runs a function in a container.
func (o *Orchestrator) Execute(ctx context.Context, function *storage.Function, event []byte) ([]byte, error) {
	// Create container
	resp, err := o.docker.ContainerCreate(ctx, &container.Config{
		Image: function.Image,
		Cmd:   []string{"/app/function"},
	}, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}
	defer o.cleanupContainer(ctx, resp.ID)

	// Start container
	if err := o.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	// Write event to container's stdin
	hijacked, err := o.docker.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to container: %v", err)
	}
	defer hijacked.Close()

	_, err = hijacked.Conn.Write(event)
	if err != nil {
		return nil, fmt.Errorf("failed to write event: %v", err)
	}
	hijacked.CloseWrite()

	// Read output
	var output bytes.Buffer
	_, err = io.Copy(&output, hijacked.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %v", err)
	}

	// Wait for container to exit
	statusCh, errCh := o.docker.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return nil, fmt.Errorf("container wait failed: %v", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return nil, fmt.Errorf("container exited with code %d", status.StatusCode)
		}
	}

	o.log.WithField("function", function.Name).Info("Function executed")
	return output.Bytes(), nil
}

// cleanupContainer removes a container
func (o *Orchestrator) cleanupContainer(ctx context.Context, containerID string) {
	if err := o.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		o.log.WithError(err).Warn("Failed to remove container")
	}
}
