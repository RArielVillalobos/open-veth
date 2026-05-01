package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/client"
)

// Manager handles communication with the Docker Daemon
type Manager struct {
	cli    *client.Client
	logger *slog.Logger
}

// NewManager creates a new orchestrator instance
func NewManager(logger *slog.Logger) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error connecting to Docker: %v", err)
	}

	return &Manager{cli: cli, logger: logger}, nil
}

// GetDockerClient returns the internal Docker client
func (m *Manager) GetDockerClient() *client.Client {
	return m.cli
}

// TestConnection checks if Docker daemon is responsive
func (m *Manager) TestConnection(ctx context.Context) error {
	_, err := m.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("could not connect to Docker: %v. Is Docker running?", err)
	}

	m.logger.Info("docker connection established")
	return nil
}
