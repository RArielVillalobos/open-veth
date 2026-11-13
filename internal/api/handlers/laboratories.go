package handlers

import (
	"context"
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
	if !h.lockLabOperation(c) {
		return
	}
	defer h.labOpMu.Unlock()
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
	if !h.lockLabOperation(c) {
		return
	}
	defer h.labOpMu.Unlock()
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

	if !h.lockLabOperation(c) {
		return
	}
	defer h.labOpMu.Unlock()
	if _, ok := h.Repo.GetLaboratory(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}
	if _, err := h.cleanupLabResources(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.DeleteLaboratory(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// SaveLabState captures and persists the current IP configuration of all nodes in a lab
func (h *Handler) SaveLabState(c *gin.Context) {
	if !h.lockLabOperation(c) {
		return
	}
	defer h.labOpMu.Unlock()
	labID := c.Param("id")
	ctx := c.Request.Context()

	lab, ok := h.Repo.GetLaboratory(labID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	h.Logger.Info("saving lab state", "id", labID, "name", lab.Name)

	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for save state", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.hydrateNodes(nodes)

	// Detect early if there are any running containers. If none, return 409
	// before checking Manager, so tests without Docker get the right error.
	// Use Status (persisted by the Docker watcher) rather than ContainerID,
	// because stopped containers retain their ContainerID in RuntimeStore.
	hasRunning := false
	for _, node := range nodes {
		if node.Status == models.NodeStatusRunning {
			hasRunning = true
			break
		}
	}
	if !hasRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "laboratory has no running containers; activate it first"})
		return
	}

	if h.Manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server not fully initialized"})
		return
	}

	configs, routeConfigs, captured, err := captureLabState(ctx, h.Manager, h.Repo, labID, nodes)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if !captured {
		c.JSON(http.StatusConflict, gin.H{"error": "laboratory has no readable running containers"})
		return
	}

	if err := h.Repo.SaveLabConfigs(labID, configs, routeConfigs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Commit each running node's filesystem as a local snapshot image
	snapshotsSaved := 0
	for i := range nodes {
		node := &nodes[i]
		if node.ContainerID == "" || node.Status != models.NodeStatusRunning {
			continue
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

// CleanupLaboratory removes all nodes and links from a specific laboratory
func (h *Handler) CleanupLaboratory(c *gin.Context) {
	labID := c.Param("id")
	ctx := c.Request.Context()

	if !h.lockLabOperation(c) {
		return
	}
	defer h.labOpMu.Unlock()
	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}
	links, err := h.Repo.ListLinksByLab(labID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	deleted, err := h.cleanupLabResources(ctx, labID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "nodes_deleted": deleted})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "laboratory cleaned", "nodes_deleted": deleted, "links_deleted": len(links)})
}

// ActivateLaboratory asynchronously stops all containers and revives the specified lab.
// Returns 202 immediately; completion is reported via the /events WebSocket as "lab:activated".
func (h *Handler) ActivateLaboratory(c *gin.Context) {
	labID := c.Param("id")

	if !h.lockLabOperation(c) {
		return
	}
	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		h.labOpMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
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
