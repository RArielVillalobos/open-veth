package handlers

import (
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
)

// ListLinks returns all links, optionally filtered by lab_id
func (h *Handler) ListLinks(c *gin.Context) {
	labID := c.Query("lab_id")
	var links []models.Link
	var err error

	if labID != "" {
		links, err = h.Repo.ListLinksByLab(labID)
	} else {
		links, err = h.Repo.ListLinks()
	}

	if err != nil {
		h.Logger.Error("failed to list links", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, links)
}

// CreateLink creates a veth pair between two nodes
func (h *Handler) CreateLink(c *gin.Context) {
	var link models.Link
	if err := c.ShouldBindJSON(&link); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default LabID if not provided
	if link.LabID == "" {
		link.LabID = "lab-1"
	}

	// Validate interface names
	if err := models.ValidateInterfaceName(link.SourceInt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source interface name"})
		return
	}
	if err := models.ValidateInterfaceName(link.TargetInt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target interface name"})
		return
	}

	// Validate link ID length for veth name generation
	if err := models.ValidateLinkID(link.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source, okS := h.Repo.GetNode(link.SourceID)
	target, okT := h.Repo.GetNode(link.TargetID)

	if !okS || !okT {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source or target node not found"})
		return
	}

	// Validation: Check for existing link between these nodes
	existingLinks, _ := h.Repo.ListLinksByLab(link.LabID)
	for _, l := range existingLinks {
		// Check both directions
		if (l.SourceID == link.SourceID && l.TargetID == link.TargetID) ||
			(l.SourceID == link.TargetID && l.TargetID == link.SourceID) {
			c.JSON(http.StatusConflict, gin.H{"error": "link already exists between these nodes"})
			return
		}
	}

	// Fetch FRESH PIDs
	ctx := c.Request.Context()
	srcPID, errS := h.Manager.GetNodePID(ctx, source.ContainerID)
	tgtPID, errT := h.Manager.GetNodePID(ctx, target.ContainerID)

	if errS != nil || errT != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nodes must be running to create a link"})
		return
	}

	h.Logger.Info("creating link",
		"source", source.Name, "source_int", link.SourceInt,
		"target", target.Name, "target_int", link.TargetInt)

	if err := h.Network.CreateLink(link, srcPID, tgtPID); err != nil {
		h.Logger.Error("failed to create link", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Attach interfaces to bridge if nodes are Switches
	if source.Type == models.SWITCH {
		if err := h.Manager.AttachInterfaceToBridge(ctx, source.ContainerID, link.SourceInt); err != nil {
			h.Logger.Warn("failed to attach interface to switch",
				"interface", link.SourceInt, "switch", source.Name, "error", err)
		}
	}
	if target.Type == models.SWITCH {
		if err := h.Manager.AttachInterfaceToBridge(ctx, target.ContainerID, link.TargetInt); err != nil {
			h.Logger.Warn("failed to attach interface to switch",
				"interface", link.TargetInt, "switch", target.Name, "error", err)
		}
	}

	if err := h.Repo.SaveLink(link); err != nil {
		h.Logger.Error("failed to save link to DB", "id", link.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist link"})
		return
	}
	h.Logger.Info("link created", "id", link.ID)
	c.JSON(http.StatusCreated, link)
}

// DeleteLink removes a veth pair
func (h *Handler) DeleteLink(c *gin.Context) {
	id := c.Param("id")

	// 1. Get Link details before deleting
	link, found := h.Repo.GetLink(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}

	h.Logger.Info("deleting link", "id", id, "source_int", link.SourceInt, "target_int", link.TargetInt)

	// 2. Physically remove interfaces from containers
	source, okS := h.Repo.GetNode(link.SourceID)
	target, okT := h.Repo.GetNode(link.TargetID)

	ctx := c.Request.Context()

	// Helper to safely get PID and remove interface
	cleanupInterface := func(node models.Node, ifaceName string) {
		if node.ContainerID == "" {
			return
		}
		// Get FRESH PID from Docker, don't trust DB cache
		pid, err := h.Manager.GetNodePID(ctx, node.ContainerID)
		if err != nil {
			h.Logger.Warn("could not get PID for node (might be stopped)",
				"node", node.Name, "error", err)
			return
		}

		if err := h.Network.RemoveInterface(pid, ifaceName); err != nil {
			h.Logger.Warn("failed to cleanup interface",
				"interface", ifaceName, "node", node.Name, "error", err)
		}
	}

	if okS {
		cleanupInterface(source, link.SourceInt)
	}
	if okT {
		cleanupInterface(target, link.TargetInt)
	}

	// 3. Delete from DB
	h.Repo.DeleteLink(id)
	c.Status(http.StatusNoContent)
}
