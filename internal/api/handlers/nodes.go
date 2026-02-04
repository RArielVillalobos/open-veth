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

	// Default LabID if not provided
	if node.LabID == "" {
		node.LabID = "lab-1"
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

	node.ContainerID = containerID
	node.PID = pid
	h.Repo.SaveNode(node)

	h.Logger.Info("node created", "name", node.Name, "container_id", containerID[:12])
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
	h.Repo.SaveNode(node)

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

	h.Logger.Info("deleting node", "name", node.Name, "id", id)

	if err := h.Manager.DeleteNode(c.Request.Context(), node.Name); err != nil {
		h.Logger.Warn("failed to delete container", "name", node.Name, "error", err)
	}

	h.Repo.DeleteNode(id)
	c.Status(http.StatusNoContent)
}

// GetNodeInterfaces returns live interface information for a node
func (h *Handler) GetNodeInterfaces(c *gin.Context) {
	id := c.Param("id")
	node, found := h.Repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	if node.ContainerID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
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

// GetNodeRoutes returns the routing table for a node
func (h *Handler) GetNodeRoutes(c *gin.Context) {
	id := c.Param("id")
	node, found := h.Repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	if node.ContainerID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
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
