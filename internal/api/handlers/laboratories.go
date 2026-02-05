package handlers

import (
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
	h.Repo.SaveLaboratory(lab)

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

	// 1. Get all nodes for this lab
	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for cleanup", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. Delete containers and DB records for each node
	deletedNodes := 0
	for _, node := range nodes {
		if node.ContainerID != "" {
			if err := h.Manager.DeleteNode(ctx, node.ContainerID); err != nil {
				h.Logger.Warn("failed to delete container during lab cleanup", "node", node.Name, "error", err)
			}
		}
		if err := h.Repo.DeleteNode(node.ID); err != nil {
			h.Logger.Warn("failed to delete node from DB", "node", node.ID, "error", err)
		} else {
			deletedNodes++
		}
	}

	// 3. Delete all links for this lab
	links, _ := h.Repo.ListLinksByLab(labID)
	deletedLinks := 0
	for _, link := range links {
		if err := h.Repo.DeleteLink(link.ID); err != nil {
			h.Logger.Warn("failed to delete link from DB", "link", link.ID, "error", err)
		} else {
			deletedLinks++
		}
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

	// 2. NUKE: Delete ALL running containers to free resources
	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err == nil {
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

		// Update Runtime Info in Struct (PID, ID)
		pid, _ := h.Manager.GetNodePID(ctx, containerID)
		nodes[i].ContainerID = containerID
		nodes[i].PID = pid

		// CRITICAL: Persist the new ContainerID and PID to DB
		h.Repo.SaveNode(nodes[i])
	}

	// 4. Rebuild Links
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
				if srcNode.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, srcNode.ContainerID, l.SourceInt)
				}
				if tgtNode.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, tgtNode.ContainerID, l.TargetInt)
				}
			}
		}
	}

	h.Logger.Info("laboratory activated", "id", labID, "nodes_revived", len(nodes))
	c.JSON(http.StatusOK, gin.H{"message": "laboratory activated", "nodes_revived": len(nodes)})
}
