package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// ImageExists returns true if the given image exists in the local Docker daemon.
func (m *Manager) ImageExists(ctx context.Context, imageName string) bool {
	_, _, err := m.cli.ImageInspectWithRaw(ctx, imageName)
	return err == nil
}

// CommitNode creates a local Docker image from a running container, capturing its current filesystem state.
func (m *Manager) CommitNode(ctx context.Context, containerID, imageName string) error {
	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	m.logger.Info("committing node snapshot", "container", shortID, "image", imageName)
	_, err := m.cli.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: imageName,
	})
	if err != nil {
		return fmt.Errorf("error committing node %s: %v", shortID, err)
	}
	return nil
}

// RemoveImage removes a local Docker image. Ignores not-found errors.
func (m *Manager) RemoveImage(ctx context.Context, imageName string) {
	_, err := m.cli.ImageRemove(ctx, imageName, image.RemoveOptions{Force: true})
	if err != nil && !client.IsErrNotFound(err) {
		m.logger.Warn("failed to remove snapshot image", "image", imageName, "error", err)
	}
}

// RemoveAllSnapshotImages removes all local Docker images with the openveth-snapshot prefix.
func (m *Manager) RemoveAllSnapshotImages(ctx context.Context) int {
	images, err := m.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		m.logger.Warn("failed to list images for snapshot cleanup", "error", err)
		return 0
	}
	removed := 0
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if strings.HasPrefix(tag, "openveth-snapshot:") {
				if _, err := m.cli.ImageRemove(ctx, img.ID, image.RemoveOptions{Force: true}); err != nil {
					m.logger.Warn("failed to remove snapshot image", "image", tag, "error", err)
				} else {
					removed++
				}
				break
			}
		}
	}
	return removed
}
