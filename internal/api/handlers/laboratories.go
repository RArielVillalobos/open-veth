package handlers

import (
	"context"
	"fmt"
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

	h.Logger.Info("deleting laboratory", "id", id)

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

	h.Logger.Info("lab state saved", "id", labID, "ips_saved", len(configs), "routes_saved", len(routeConfigs))
	c.JSON(http.StatusOK, gin.H{
		"message":      "state saved",
		"ips_saved":    len(configs),
		"routes_saved": len(routeConfigs),
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
				if len(addr.Address) > 4 && addr.Address[:4] == "fe80" {
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
		if h.Manager != nil {
			h.Manager.RemoveNodeVolumes(ctx, node.Name)
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

// ActivateLaboratory stops all containers and revives the specified lab
func (h *Handler) ActivateLaboratory(c *gin.Context) {
	labID := c.Param("id")
	ctx := c.Request.Context()

	// 1. Verify Lab Exists
	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	h.Logger.Info("activating laboratory", "id", labID)

	// 2. Acquire lock for the entire save-nuke-rebuild sequence
	h.labOpMu.Lock()
	defer h.labOpMu.Unlock()

	// 3. Save current active lab state before destroying containers
	if err := h.saveAllLabsStateLocked(ctx); err != nil {
		h.Logger.Warn("failed to save state before lab switch", "error", err)
	}

	// 4. NUKE: Delete ALL running containers to free resources (state already saved in step 3)
	h.Runtime.Clear()
	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err == nil {
		for _, container := range containers {
			if err := h.Manager.DeleteNode(ctx, container.ID); err != nil {
				h.Logger.Warn("failed to stop container during lab activation",
					"container", container.ID[:12], "error", err)
			}
		}
	}

	// 5. Rebuild Nodes
	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to fetch nodes for lab", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch nodes: " + err.Error()})
		return
	}

	for i, n := range nodes {
		// Re-create container
		containerID, err := h.Manager.CreateNode(ctx, n)
		if err != nil {
			h.Logger.Error("failed to revive node", "node", n.Name, "error", err)
			continue
		}

		// Update Runtime Info
		pid, err := h.Manager.GetNodePID(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get PID for revived node", "node", n.Name, "error", err)
		}
		h.Runtime.Set(n.ID, containerID, pid)
		nodes[i].ContainerID = containerID
		nodes[i].PID = pid
	}

	// 6. Rebuild Links
	links, err := h.Repo.ListLinksByLab(labID)
	if err != nil {
		h.Logger.Warn("failed to fetch links for lab", "lab", labID, "error", err)
	}

	for _, l := range links {
		// Find source and target PIDs from our fresh `nodes` slice
		var srcNode, tgtNode models.Node
		foundS, foundT := false, false

		for _, n := range nodes {
			if n.ID == l.SourceID {
				srcNode = n
				foundS = true
			}
			if n.ID == l.TargetID {
				tgtNode = n
				foundT = true
			}
		}

		if foundS && foundT && srcNode.PID > 0 && tgtNode.PID > 0 {
			if err := h.Network.CreateLink(l, srcNode.PID, tgtNode.PID); err != nil {
				h.Logger.Error("failed to revive link", "link", l.ID, "error", err)
			} else {
				// Re-attach bridges if needed
				if models.NeedsBridge(srcNode.Type) {
					_ = h.Manager.AttachInterfaceToBridge(ctx, srcNode.ContainerID, l.SourceInt, srcNode.Type)
				}
				if models.NeedsBridge(tgtNode.Type) {
					_ = h.Manager.AttachInterfaceToBridge(ctx, tgtNode.ContainerID, l.TargetInt, tgtNode.Type)
				}
			}
		}
	}

	// 7. Restore saved IP configurations
	savedConfigs, err := h.Repo.GetInterfaceConfigsByLab(labID)
	if err != nil {
		h.Logger.Warn("failed to fetch saved configs", "lab", labID, "error", err)
	}

	// Build node lookup map for O(1) access
	nodeMap := make(map[string]models.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	restoredIPs := 0
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

	// 8. Restore saved route configurations (must happen after IPs are restored)
	savedRoutes, err := h.Repo.GetRouteConfigsByLab(labID)
	if err != nil {
		h.Logger.Warn("failed to fetch saved routes", "lab", labID, "error", err)
	}

	restoredRoutes := 0
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

	h.Logger.Info("laboratory activated", "id", labID, "nodes_revived", len(nodes), "ips_restored", restoredIPs, "routes_restored", restoredRoutes)
	c.JSON(http.StatusOK, gin.H{
		"message":         "laboratory activated",
		"nodes_revived":   len(nodes),
		"ips_restored":    restoredIPs,
		"routes_restored": restoredRoutes,
	})
}
