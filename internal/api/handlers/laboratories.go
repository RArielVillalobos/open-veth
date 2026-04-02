package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
)

// ListLaboratories returns all available laboratories
func (h *Handler) ListLaboratories(c *gin.Context) {
	labs, err := h.Repo.ListLaboratories()
	if err != nil {
		h.Logger.Error("failed to list laboratories", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, labs)
}

// CreateLaboratory creates a new laboratory
func (h *Handler) CreateLaboratory(c *gin.Context) {
	var lab models.Laboratory
	if err := c.ShouldBindJSON(&lab); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if lab.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lab id is required"})
		return
	}

	h.Logger.Info("creating laboratory", "id", lab.ID, "name", lab.Name)

	if err := h.Repo.SaveLaboratory(lab); err != nil {
		h.Logger.Error("failed to create laboratory", "id", lab.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, lab)
}

// UpdateLaboratory updates laboratory metadata
func (h *Handler) UpdateLaboratory(c *gin.Context) {
	id := c.Param("id")
	var data struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lab, found := h.Repo.GetLaboratory(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	lab.Name = data.Name
	if err := h.Repo.SaveLaboratory(lab); err != nil {
		h.Logger.Error("failed to update laboratory", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update laboratory"})
		return
	}

	h.Logger.Info("laboratory updated", "id", id, "name", data.Name)
	c.JSON(http.StatusOK, lab)
}

// DeleteLaboratory removes a laboratory and all its resources
func (h *Handler) DeleteLaboratory(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	h.Logger.Info("deleting laboratory", "id", id)

	// Remove snapshot images for all nodes before deleting from DB
	if nodes, err := h.Repo.ListNodesByLab(id); err == nil {
		for _, node := range nodes {
			if node.SnapshotImage != "" {
				h.Manager.RemoveImage(ctx, node.SnapshotImage)
			}
		}
	}

	if err := h.Repo.DeleteLaboratory(id); err != nil {
		h.Logger.Error("failed to delete laboratory", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// SaveLabState captures and persists the current IP configuration of all nodes in a lab
func (h *Handler) SaveLabState(c *gin.Context) {
	labID := c.Param("id")
	ctx := c.Request.Context()

	lab, ok := h.Repo.GetLaboratory(labID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	h.Logger.Info("saving lab state", "id", labID, "name", lab.Name)

	h.labOpMu.Lock()
	defer h.labOpMu.Unlock()

	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for save state", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.hydrateNodes(nodes)

	configs, routeConfigs, hasRunning := h.captureLabState(ctx, labID, nodes)
	if !hasRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "laboratory has no running containers; activate it first"})
		return
	}

	if err := h.Repo.SaveInterfaceConfigs(labID, configs); err != nil {
		h.Logger.Error("failed to save interface configs", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.SaveRouteConfigs(labID, routeConfigs); err != nil {
		h.Logger.Error("failed to save route configs", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Commit each running node's filesystem as a local snapshot image
	snapshotsSaved := 0
	for i := range nodes {
		node := &nodes[i]
		if node.ContainerID == "" {
			continue
		}
		if node.SnapshotImage != "" {
			h.Manager.RemoveImage(ctx, node.SnapshotImage)
		}
		imageName := models.SnapshotImageName(node.ID)
		if err := h.Manager.CommitNode(ctx, node.ContainerID, imageName); err != nil {
			h.Logger.Warn("failed to commit node snapshot", "node", node.Name, "error", err)
			continue
		}
		node.SnapshotImage = imageName
		if err := h.Repo.SaveNode(*node); err != nil {
			h.Logger.Warn("failed to save snapshot image to db", "node", node.Name, "error", err)
		} else {
			snapshotsSaved++
		}
	}

	h.Logger.Info("lab state saved", "id", labID, "ips_saved", len(configs), "routes_saved", len(routeConfigs), "snapshots", snapshotsSaved)
	c.JSON(http.StatusOK, gin.H{
		"message":         "state saved",
		"ips_saved":       len(configs),
		"routes_saved":    len(routeConfigs),
		"snapshots_saved": snapshotsSaved,
	})
}

// captureLabState reads live IP and route configuration from running containers.
// Nodes must be hydrated before calling. Returns hasRunning=false if no containers
// are running, allowing the caller to skip the save and preserve existing DB state.
func (h *Handler) captureLabState(ctx context.Context, labID string, nodes []models.Node) (
	configs []models.InterfaceConfig, routeConfigs []models.RouteConfig, hasRunning bool,
) {
	for _, node := range nodes {
		if node.ContainerID == "" {
			continue
		}
		hasRunning = true

		ifaces, err := h.Manager.GetNodeInterfaces(ctx, node.ContainerID)
		if err != nil {
			h.Logger.Warn("failed to get interfaces for node", "node", node.Name, "error", err)
			continue
		}

		for _, iface := range ifaces {
			if iface.Name == "lo" || iface.Name == "mgmt0" {
				continue
			}
			for _, addr := range iface.IPAddresses {
				if ip := net.ParseIP(addr.Address); ip != nil && ip.IsLinkLocalUnicast() {
					continue
				}
				configs = append(configs, models.InterfaceConfig{
					LabID:     labID,
					NodeID:    node.ID,
					Interface: iface.Name,
					Address:   fmt.Sprintf("%s/%d", addr.Address, addr.Prefix),
				})
			}
		}

		routes, err := h.Manager.GetNodeRoutes(ctx, node.ContainerID)
		if err != nil {
			h.Logger.Warn("failed to get routes for node", "node", node.Name, "error", err)
			continue
		}

		for _, r := range routes {
			if r.Protocol == "kernel" {
				continue
			}
			routeConfigs = append(routeConfigs, models.RouteConfig{
				LabID:   labID,
				NodeID:  node.ID,
				Dst:     r.Dst,
				Gateway: r.Gateway,
				Dev:     r.Dev,
			})
		}
	}
	return
}

// CleanupLaboratory removes all nodes and links from a specific laboratory
func (h *Handler) CleanupLaboratory(c *gin.Context) {
	labID := c.Param("id")
	ctx := c.Request.Context()

	// Verify Lab Exists
	lab, ok := h.Repo.GetLaboratory(labID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	h.Logger.Info("cleaning up laboratory", "id", labID, "name", lab.Name)

	// 1. Get all nodes and links for this lab
	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for cleanup", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.hydrateNodes(nodes)

	links, _ := h.Repo.ListLinksByLab(labID)
	deletedLinks := len(links) // cascade via DeleteNode will handle these

	// 2. Delete containers, volumes, and DB records for each node (cascade deletes links + configs)
	deletedNodes := 0
	for _, node := range nodes {
		if node.ContainerID != "" {
			if err := h.Manager.DeleteNode(ctx, node.ContainerID); err != nil {
				h.Logger.Warn("failed to delete container during lab cleanup", "node", node.Name, "error", err)
			}
		}
		if h.Manager != nil && node.SnapshotImage != "" {
			h.Manager.RemoveImage(ctx, node.SnapshotImage)
		}
		if err := h.Repo.DeleteNode(node.ID); err != nil {
			h.Logger.Warn("failed to delete node from DB", "node", node.ID, "error", err)
		} else {
			deletedNodes++
		}
		h.Runtime.Delete(node.ID)
	}

	h.Logger.Info("laboratory cleanup completed", "id", labID, "nodes_deleted", deletedNodes, "links_deleted", deletedLinks)
	c.JSON(http.StatusOK, gin.H{
		"message":       "laboratory cleaned",
		"nodes_deleted": deletedNodes,
		"links_deleted": deletedLinks,
	})
}

// ActivateLaboratory asynchronously stops all containers and revives the specified lab.
// Returns 202 immediately; completion is reported via the /events WebSocket as "lab:activated".
func (h *Handler) ActivateLaboratory(c *gin.Context) {
	labID := c.Param("id")

	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	// Non-blocking lock: reject if another lab operation is already in progress
	if !h.labOpMu.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"error": "another lab operation is in progress"})
		return
	}

	h.Logger.Info("activating laboratory (async)", "id", labID)
	c.JSON(http.StatusAccepted, gin.H{"message": "activation started", "lab_id": labID})

	// Run activation in background; use a fresh context since the request context
	// will be cancelled as soon as the HTTP handler returns.
	go func() {
		defer h.labOpMu.Unlock()
		h.runActivation(context.Background(), labID)
	}()
}

// runActivation performs the full lab rebuild sequence and broadcasts the result.
// Must be called while holding labOpMu.
func (h *Handler) runActivation(ctx context.Context, labID string) {
	defer func() {
		if r := recover(); r != nil {
			h.Logger.Error("panic during lab activation", "lab", labID, "panic", r)
			h.BroadcastLabActivationFailed(labID, "internal error during activation")
		}
	}()

	// 1. Save current active lab state before destroying containers
	if err := h.saveAllLabsStateLocked(ctx); err != nil {
		h.Logger.Warn("failed to save state before lab switch", "error", err)
	}

	// 2. NUKE: Delete ALL running containers to free resources
	h.Runtime.Clear()
	if containers, err := h.Manager.GetOpenVethContainers(ctx); err == nil {
		for _, container := range containers {
			if err := h.Manager.DeleteNode(ctx, container.ID); err != nil {
				h.Logger.Warn("failed to stop container during lab activation",
					"container", container.ID[:12], "error", err)
			}
		}
	}

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
			nodeMap[n.ID] = nodes[i]
			continue
		}

		pid, err := h.Manager.GetNodePID(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get PID for revived node", "node", n.Name, "error", err)
		}
		h.Runtime.Set(n.ID, containerID, pid)
		nodes[i].ContainerID = containerID
		nodes[i].PID = pid
		h.storeMonitorPorts(ctx, &nodes[i], containerID)
		nodeMap[n.ID] = nodes[i]
		nodesRevived++
	}

	// 4. Rebuild Links
	if links, err := h.Repo.ListLinksByLab(labID); err != nil {
		h.Logger.Warn("failed to fetch links for lab", "lab", labID, "error", err)
	} else {
		for _, l := range links {
			srcNode, foundS := nodeMap[l.SourceID]
			tgtNode, foundT := nodeMap[l.TargetID]
			if foundS && foundT && srcNode.PID > 0 && tgtNode.PID > 0 {
				if err := h.Network.CreateLink(l, srcNode.PID, tgtNode.PID); err != nil {
					h.Logger.Error("failed to revive link", "link", l.ID, "error", err)
				} else {
					if models.NeedsBridge(srcNode.Type) {
						_ = h.Manager.AttachInterfaceToBridge(ctx, srcNode.ContainerID, l.SourceInt, srcNode.Type)
					}
					if models.NeedsBridge(tgtNode.Type) {
						_ = h.Manager.AttachInterfaceToBridge(ctx, tgtNode.ContainerID, l.TargetInt, tgtNode.Type)
					}
				}
			}
		}
	}

	// 5. Restore saved IP configurations
	restoredIPs := 0
	if savedConfigs, err := h.Repo.GetInterfaceConfigsByLab(labID); err != nil {
		h.Logger.Warn("failed to fetch saved configs", "lab", labID, "error", err)
	} else {
		for _, cfg := range savedConfigs {
			if n, ok := nodeMap[cfg.NodeID]; ok && n.ContainerID != "" {
				if err := h.Manager.ConfigureInterface(ctx, n.ContainerID, cfg.Interface, cfg.Address); err != nil {
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
		h.Logger.Warn("failed to fetch saved routes", "lab", labID, "error", err)
	} else {
		for _, cfg := range savedRoutes {
			if n, ok := nodeMap[cfg.NodeID]; ok && n.ContainerID != "" {
				if err := h.Manager.ConfigureRoute(ctx, n.ContainerID, cfg.Dst, cfg.Gateway, cfg.Dev); err != nil {
					h.Logger.Warn("failed to restore route",
						"node", n.Name, "dst", cfg.Dst, "gw", cfg.Gateway, "error", err)
				} else {
					restoredRoutes++
				}
			}
		}
	}

	h.Logger.Info("laboratory activated", "id", labID,
		"nodes_revived", nodesRevived, "ips_restored", restoredIPs, "routes_restored", restoredRoutes)
	h.BroadcastLabActivated(labID, nodesRevived, restoredIPs, restoredRoutes)
}
