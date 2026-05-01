package handlers

import (
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
)

// ListNodes returns all nodes, optionally filtered by lab_id
func (h *Handler) ListNodes(c *gin.Context) {
	labID := c.Query("lab_id")
	var nodes []models.Node
	var err error

	if labID != "" {
		nodes, err = h.Repo.ListNodesByLab(labID)
	} else {
		nodes, err = h.Repo.ListNodes()
	}

	if err != nil {
		h.Logger.Error("failed to list nodes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hydrateNodes(nodes)

	// If real-time info is requested
	if c.Query("live") == "true" {
		for i := range nodes {
			if nodes[i].ContainerID != "" {
				if ifaces, err := h.Manager.GetNodeInterfaces(c.Request.Context(), nodes[i].ContainerID); err == nil {
					nodes[i].Interfaces = ifaces
				}
			}
		}
	}

	c.JSON(http.StatusOK, nodes)
}

// CreateNode creates a new node and its container
func (h *Handler) CreateNode(c *gin.Context) {
	var node models.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if node.LabID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lab_id is required"})
		return
	}

	// SECURITY: Override user-provided image with official image
	node.Image = models.GetImageForType(node.Type)

	h.Logger.Info("creating node", "name", node.Name, "type", node.Type, "lab", node.LabID)

	containerID, err := h.Manager.CreateNode(c.Request.Context(), node)
	if err != nil {
		h.Logger.Error("failed to create node", "name", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pid, err := h.Manager.GetNodePID(c.Request.Context(), containerID)
	if err != nil {
		h.Logger.Error("failed to get node PID", "name", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Persist config (gorm:"-" fields are NOT saved)
	node.Status = models.NodeStatusRunning
	if err := h.Repo.SaveNode(node); err != nil {
		h.Logger.Error("failed to save node to DB, cleaning up container", "name", node.Name, "error", err)
		_ = h.Manager.DeleteNode(c.Request.Context(), node.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist node"})
		return
	}

	// Store runtime state in memory
	h.Runtime.Set(node.ID, containerID, pid)
	node.ContainerID = containerID
	node.PID = pid

	// For MONITOR nodes, retrieve and store the dynamically assigned host ports
	h.storeMonitorPorts(c.Request.Context(), &node, containerID)

	h.Logger.Info("node created", "name", node.Name, "container_id", containerID[:12])

	// Broadcast node creation event
	h.BroadcastNodeCreated(node.ID, node.LabID, string(node.Type))

	c.JSON(http.StatusCreated, node)
}

// UpdateNodePosition updates the canvas position of a node
func (h *Handler) UpdateNodePosition(c *gin.Context) {
	id := c.Param("id")
	var pos struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, found := h.Repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	node.X = pos.X
	node.Y = pos.Y
	if err := h.Repo.SaveNode(node); err != nil {
		h.Logger.Error("failed to save node position", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save position"})
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteNode removes a node and its container
func (h *Handler) DeleteNode(c *gin.Context) {
	id := c.Param("id")
	node, found := h.Repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	h.hydrateNode(&node)

	h.Logger.Info("deleting node", "name", node.Name, "id", id)
	ctx := c.Request.Context()

	// 1. Clean up link interfaces on PEER nodes before cascade delete
	links, err := h.Repo.ListLinksByNode(id)
	if err != nil {
		h.Logger.Warn("failed to list links for node cleanup", "node", node.Name, "error", err)
	} else {
		for _, link := range links {
			var peerNodeID, peerInt string
			if link.SourceID == id {
				peerNodeID = link.TargetID
				peerInt = link.TargetInt
			} else {
				peerNodeID = link.SourceID
				peerInt = link.SourceInt
			}

			peerNode, ok := h.Repo.GetNode(peerNodeID)
			if !ok {
				continue
			}
			h.hydrateNode(&peerNode)
			if peerNode.ContainerID == "" {
				continue
			}
			pid, err := h.Manager.GetNodePID(ctx, peerNode.ContainerID)
			if err != nil {
				continue
			}
			if err := h.Network.RemoveInterface(pid, peerInt); err != nil {
				h.Logger.Warn("failed to cleanup peer interface",
					"interface", peerInt, "peer", peerNode.Name, "error", err)
			}
		}
	}

	// 2. Delete the container and its volumes
	if node.ContainerID != "" {
		if err := h.Manager.DeleteNode(ctx, node.Name); err != nil {
			h.Logger.Warn("failed to delete container", "name", node.Name, "error", err)
		}
	}
	if h.Manager != nil && node.SnapshotImage != "" {
		h.Manager.RemoveImage(ctx, node.SnapshotImage)
	}

	// 3. Delete from DB (cascade removes links + interface configs)
	if err := h.Repo.DeleteNode(id); err != nil {
		h.Logger.Error("failed to delete node from DB", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete node"})
		return
	}

	// 4. Remove runtime state
	h.Runtime.Delete(id)

	c.Status(http.StatusNoContent)
}

// StopNode stops a running node container without deleting it
func (h *Handler) StopNode(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := h.Manager.StopNode(ctx, node.Name); err != nil {
		h.Logger.Error("failed to stop node", "name", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	node.Status = models.NodeStatusStopped
	if err := h.Repo.SaveNode(node); err != nil {
		h.Logger.Warn("failed to persist node status", "name", node.Name, "error", err)
	}

	h.BroadcastNodeStopped(node.ID, node.LabID)
	h.Logger.Info("node stopped", "name", node.Name)
	c.JSON(http.StatusOK, node)
}

// StartNode starts a stopped node container
func (h *Handler) StartNode(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	// Use CreateNode which handles all post-restart setup:
	// WaitForReady, eth0→docker0 rename, bridge init, cloud NAT, linux gateway
	if _, err := h.Manager.CreateNode(ctx, node); err != nil {
		h.Logger.Error("failed to start node", "name", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pid, err := h.Manager.GetNodePID(ctx, node.ContainerID)
	if err != nil {
		h.Logger.Warn("failed to get node PID after start", "name", node.Name, "error", err)
	} else {
		h.Runtime.Set(node.ID, node.ContainerID, pid)
		node.PID = pid
	}

	// Reattach veth pairs (network namespace is fresh after restart)
	// Also restore peer IPs: when a veth pair is recreated, the peer's end is also
	// replaced with a new interface that loses its IP address.
	labIfaces, _ := h.Repo.GetInterfaceConfigsByLab(node.LabID)
	if links, err := h.Repo.ListLinksByNode(node.ID); err == nil {
		for _, l := range links {
			peerID := l.TargetID
			nodeIface, peerIface := l.SourceInt, l.TargetInt
			if l.TargetID == node.ID {
				peerID = l.SourceID
				nodeIface, peerIface = l.TargetInt, l.SourceInt
			}

			peerState, ok := h.Runtime.Get(peerID)
			if !ok || peerState.PID == 0 {
				h.Logger.Warn("peer not running, skipping link restore", "link", l.ID)
				continue
			}
			peerNode, found := h.Repo.GetNode(peerID)
			if !found {
				continue
			}
			peerNode.ContainerID = peerState.ContainerID
			peerNode.PID = peerState.PID

			srcPID, tgtPID := node.PID, peerState.PID
			if l.TargetID == node.ID {
				srcPID, tgtPID = peerState.PID, node.PID
			}

			if err := h.Network.CreateLink(l, srcPID, tgtPID); err != nil {
				h.Logger.Warn("failed to restore link after power on", "link", l.ID, "error", err)
				continue
			}

			// Bridge attachment for powered-on node
			if models.NeedsBridge(node.Type) {
				_ = h.Manager.AttachInterfaceToBridge(ctx, node.ContainerID, nodeIface, node.Type)
			}
			// Bridge attachment for peer (e.g. peer is SWITCH/HUB)
			if models.NeedsBridge(peerNode.Type) {
				_ = h.Manager.AttachInterfaceToBridge(ctx, peerNode.ContainerID, peerIface, peerNode.Type)
			}

			// Restore peer's IP on this interface (its veth end was also recreated)
			for _, cfg := range labIfaces {
				if cfg.NodeID == peerID && cfg.Interface == peerIface {
					if err := h.Manager.ConfigureInterface(ctx, peerNode.ContainerID, cfg.Interface, cfg.Address); err != nil {
						h.Logger.Warn("failed to restore peer IP after power on", "peer", peerNode.Name, "iface", cfg.Interface, "error", err)
					}
					break
				}
			}
		}
	}

	// Reapply saved IP configurations for the powered-on node
	for _, cfg := range labIfaces {
		if cfg.NodeID != node.ID {
			continue
		}
		if err := h.Manager.ConfigureInterface(ctx, node.ContainerID, cfg.Interface, cfg.Address); err != nil {
			h.Logger.Warn("failed to restore IP after power on", "node", node.Name, "iface", cfg.Interface, "error", err)
		}
	}
	if routes, err := h.Repo.GetRouteConfigsByLab(node.LabID); err == nil {
		for _, cfg := range routes {
			if cfg.NodeID != node.ID {
				continue
			}
			if err := h.Manager.ConfigureRoute(ctx, node.ContainerID, cfg.Dst, cfg.Gateway, cfg.Dev); err != nil {
				h.Logger.Warn("failed to restore route after power on", "node", node.Name, "dst", cfg.Dst, "error", err)
			}
		}
	}

	node.Status = models.NodeStatusRunning
	if err := h.Repo.SaveNode(node); err != nil {
		h.Logger.Warn("failed to persist node status", "name", node.Name, "error", err)
	}

	h.BroadcastNodeStarted(node.ID, node.LabID)
	h.Logger.Info("node started", "name", node.Name)
	c.JSON(http.StatusOK, node)
}

// GetNodeInterfaces returns live interface information for a node
func (h *Handler) GetNodeInterfaces(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	interfaces, err := h.Manager.GetNodeInterfaces(c.Request.Context(), node.ContainerID)
	if err != nil {
		h.Logger.Error("failed to get interfaces", "node", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, interfaces)
}

// GetNodeMacTable returns the MAC address table (bridge FDB) for a switch node
func (h *Handler) GetNodeMacTable(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	entries, err := h.Manager.GetNodeMacTable(c.Request.Context(), node.ContainerID)
	if err != nil {
		h.Logger.Error("failed to get mac table", "node", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, entries)
}

// GetNodeRoutes returns the routing table for a node
func (h *Handler) GetNodeRoutes(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	routes, err := h.Manager.GetNodeRoutes(c.Request.Context(), node.ContainerID)
	if err != nil {
		h.Logger.Error("failed to get routes", "node", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, routes)
}
