package handlers

import (
	"context"
	"errors"
	"fmt"
)

// cleanupLabResources is shared by laboratory deletion, cleanup, and import.
// The caller holds labOpMu. Failed nodes remain in the repository for retry.
func (h *Handler) cleanupLabResources(ctx context.Context, labID string) (int, error) {
	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		return 0, err
	}
	h.hydrateNodes(nodes)
	deleted := 0
	var failures []error
	for _, node := range nodes {
		if h.Manager != nil {
			if err := removeNodeResources(ctx, h.Manager, node); err != nil {
				failures = append(failures, err)
				continue
			}
		} else if node.ContainerID != "" || node.SnapshotImage != "" {
			failures = append(failures, fmt.Errorf("cannot clean node %s: server not fully initialized", node.ID))
			continue
		}
		if err := h.Repo.DeleteNode(node.ID); err != nil {
			failures = append(failures, err)
			continue
		}
		h.Runtime.Delete(node.ID)
		deleted++
	}
	return deleted, errors.Join(failures...)
}
