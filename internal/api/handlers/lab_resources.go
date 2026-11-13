package handlers

import (
	"context"
	"fmt"
	"open-veth/internal/models"
)

type labResourceRemover interface {
	DeleteNode(context.Context, string) error
	RemoveImage(context.Context, string) error
}

// removeNodeResources leaves persistence untouched when Docker cleanup fails.
func removeNodeResources(ctx context.Context, remover labResourceRemover, node models.Node) error {
	if remover == nil {
		return fmt.Errorf("server not fully initialized")
	}
	containerID := node.ContainerID
	if containerID == "" {
		containerID = node.Name
	}
	if err := remover.DeleteNode(ctx, containerID); err != nil {
		return fmt.Errorf("delete container for %s: %w", node.ID, err)
	}
	if node.SnapshotImage != "" {
		if err := remover.RemoveImage(ctx, node.SnapshotImage); err != nil {
			return fmt.Errorf("delete snapshot for %s: %w", node.ID, err)
		}
	}
	return nil
}
