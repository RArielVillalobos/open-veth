package handlers

import (
	"context"
	"fmt"

	"open-veth/internal/models"
)

// runActivation performs the full lab rebuild sequence and broadcasts the result.
// Must be called while holding labOpMu.
func (h *Handler) runActivation(ctx context.Context, labID string) {
	defer func() {
		if r := recover(); r != nil {
			h.Logger.Error("panic during lab activation", "lab", labID, "panic", r)
			h.BroadcastLabActivationFailed(labID, "internal error during activation")
		}
	}()

	if h.Manager == nil || h.Network == nil {
		h.Logger.Error("cannot activate laboratory: manager or network not initialized", "lab", labID)
		h.BroadcastLabActivationFailed(labID, "server not fully initialized")
		return
	}

	// 1. Save current active lab state before destroying containers
	if err := h.saveAllLabsStateLocked(ctx); err != nil {
		h.BroadcastLabActivationFailed(labID, "failed to save current state: "+err.Error())
		return
	}

	// Stop old containers before dropping their runtime references.
	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err != nil {
		h.BroadcastLabActivationFailed(labID, err.Error())
		return
	}
	for _, container := range containers {
		if err := h.Manager.DeleteNode(ctx, container.ID); err != nil {
			h.BroadcastLabActivationFailed(labID, "failed to stop previous laboratory: "+err.Error())
			return
		}
		if ref, ok := h.Runtime.FindByContainerID(container.ID); ok {
			h.Runtime.Delete(ref.NodeID)
			if node, found := h.Repo.GetNode(ref.NodeID); found {
				node.Status = models.NodeStatusStopped
				if err := h.Repo.SaveNode(node); err != nil {
					h.BroadcastLabActivationFailed(labID, err.Error())
					return
				}
			}
		}
	}
	h.Runtime.Clear()
	allNodes, err := h.Repo.ListNodes()
	if err != nil {
		h.BroadcastLabActivationFailed(labID, err.Error())
		return
	}
	for _, node := range allNodes {
		node.Status = models.NodeStatusStopped
		if err := h.Repo.SaveNode(node); err != nil {
			h.BroadcastLabActivationFailed(labID, err.Error())
			return
		}
	}
	failures := 0

	// 3. Rebuild Nodes
	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to fetch nodes for lab", "lab", labID, "error", err)
		h.BroadcastLabActivationFailed(labID, "failed to fetch nodes")
		return
	}

	nodeMap := make(map[string]models.Node, len(nodes))
	nodesRevived := 0

	for i, n := range nodes {
		if n.SnapshotImage != "" && h.Manager.ImageExists(ctx, n.SnapshotImage) {
			n.Image = n.SnapshotImage
		}
		containerID, err := h.Manager.CreateNode(ctx, n)
		if err != nil {
			h.Logger.Error("failed to revive node", "node", n.Name, "error", err)
			failures++
			continue
		}

		pid, err := h.Manager.GetNodePID(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get PID for revived node", "node", n.Name, "error", err)
			failures++
		}
		nodes[i].Status = models.NodeStatusRunning
		if err := h.Repo.SaveNode(nodes[i]); err != nil {
			failures++
		}
		h.Runtime.Set(n.ID, containerID, pid)
		nodes[i].ContainerID = containerID
		nodes[i].PID = pid
		h.storeServicePorts(ctx, &nodes[i], containerID)
		nodeMap[n.ID] = nodes[i]
		nodesRevived++
	}

	// 4. Rebuild Links
	if links, err := h.Repo.ListLinksByLab(labID); err != nil {
		failures++
		h.Logger.Warn("failed to fetch links for lab", "lab", labID, "error", err)
	} else {
		for _, l := range links {
			srcNode, foundS := nodeMap[l.SourceID]
			tgtNode, foundT := nodeMap[l.TargetID]
			if !foundS || !foundT || srcNode.PID <= 0 || tgtNode.PID <= 0 {
				failures++
				continue
			}
			if err := h.Network.CreateLink(l, srcNode.PID, tgtNode.PID); err != nil {
				failures++
				h.Logger.Error("failed to revive link", "link", l.ID, "error", err)
			} else {
				if models.NeedsBridge(srcNode.Type) {
					if err := h.Manager.AttachInterfaceToBridge(ctx, srcNode.ContainerID, l.SourceInt, srcNode.Type); err != nil {
						failures++
						h.Logger.Error("failed to attach interface to bridge", "interface", l.SourceInt, "node", srcNode.Name, "type", srcNode.Type, "error", err)
					}
				}
				if models.NeedsBridge(tgtNode.Type) {
					if err := h.Manager.AttachInterfaceToBridge(ctx, tgtNode.ContainerID, l.TargetInt, tgtNode.Type); err != nil {
						failures++
						h.Logger.Error("failed to attach interface to bridge", "interface", l.TargetInt, "node", tgtNode.Name, "type", tgtNode.Type, "error", err)
					}
				}
			}
		}
	}

	// 5. Restore saved IP configurations
	restoredIPs := 0
	if savedConfigs, err := h.Repo.GetInterfaceConfigsByLab(labID); err != nil {
		failures++
		h.Logger.Warn("failed to fetch saved configs", "lab", labID, "error", err)
	} else {
		for _, cfg := range savedConfigs {
			if n, ok := nodeMap[cfg.NodeID]; ok && n.ContainerID != "" {
				if err := h.Manager.ConfigureInterface(ctx, n.ContainerID, cfg.Interface, cfg.Address); err != nil {
					failures++
					h.Logger.Warn("failed to restore IP config",
						"node", n.Name, "interface", cfg.Interface, "address", cfg.Address, "error", err)
				} else {
					restoredIPs++
				}
			}
		}
	}

	// 6. Restore saved route configurations (must happen after IPs are restored)
	restoredRoutes := 0
	if savedRoutes, err := h.Repo.GetRouteConfigsByLab(labID); err != nil {
		failures++
		h.Logger.Warn("failed to fetch saved routes", "lab", labID, "error", err)
	} else {
		for _, cfg := range savedRoutes {
			if n, ok := nodeMap[cfg.NodeID]; ok && n.ContainerID != "" {
				if err := h.Manager.ConfigureRoute(ctx, n.ContainerID, cfg.Dst, cfg.Gateway, cfg.Dev); err != nil {
					failures++
					h.Logger.Warn("failed to restore route",
						"node", n.Name, "dst", cfg.Dst, "gw", cfg.Gateway, "error", err)
				} else {
					restoredRoutes++
				}
			}
		}
	}

	if failures > 0 {
		h.BroadcastLabActivationFailed(labID, fmt.Sprintf("partial activation: %d operations failed; %d/%d nodes started", failures, nodesRevived, len(nodes)))
		return
	}
	h.Logger.Info("laboratory activated", "id", labID,
		"nodes_revived", nodesRevived, "ips_restored", restoredIPs, "routes_restored", restoredRoutes)
	h.BroadcastLabActivated(labID, nodesRevived, restoredIPs, restoredRoutes)
}
