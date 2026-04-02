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

	// New links are always enabled
	link.Enabled = true

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
	h.hydrateNode(&source)
	h.hydrateNode(&target)

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

	// Attach interfaces to bridge if nodes need one (switch/hub)
	if models.NeedsBridge(source.Type) {
		if err := h.Manager.AttachInterfaceToBridge(ctx, source.ContainerID, link.SourceInt, source.Type); err != nil {
			h.Logger.Warn("failed to attach interface to bridge",
				"interface", link.SourceInt, "node", source.Name, "type", source.Type, "error", err)
		}
	}
	if models.NeedsBridge(target.Type) {
		if err := h.Manager.AttachInterfaceToBridge(ctx, target.ContainerID, link.TargetInt, target.Type); err != nil {
			h.Logger.Warn("failed to attach interface to bridge",
				"interface", link.TargetInt, "node", target.Name, "type", target.Type, "error", err)
		}
	}

	if err := h.Repo.SaveLink(link); err != nil {
		h.Logger.Error("failed to save link to DB, rolling back veth pair", "id", link.ID, "error", err)
		h.Network.RemoveInterface(srcPID, link.SourceInt)
		h.Network.RemoveInterface(tgtPID, link.TargetInt)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist link"})
		return
	}
	h.Logger.Info("link created", "id", link.ID)

	// Broadcast link creation event
	h.BroadcastLinkCreated(link.ID, link.LabID, link.SourceID, link.TargetID)

	c.JSON(http.StatusCreated, link)
}

// ToggleLink enables or disables a link by bringing its interfaces up or down
func (h *Handler) ToggleLink(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, found := h.Repo.GetLink(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}

	source, okS := h.Repo.GetNode(link.SourceID)
	target, okT := h.Repo.GetNode(link.TargetID)
	if okS {
		h.hydrateNode(&source)
	}
	if okT {
		h.hydrateNode(&target)
	}

	ctx := c.Request.Context()

	setEnabled := func(node models.Node, ifaceName string) {
		if node.ContainerID == "" {
			return
		}
		pid, err := h.Manager.GetNodePID(ctx, node.ContainerID)
		if err != nil {
			h.Logger.Warn("could not get PID for node (might be stopped)",
				"node", node.Name, "error", err)
			return
		}
		if err := h.Network.SetLinkEnabled(pid, ifaceName, req.Enabled); err != nil {
			h.Logger.Warn("failed to set link state",
				"iface", ifaceName, "node", node.Name, "enabled", req.Enabled, "error", err)
		}
	}

	if okS {
		setEnabled(source, link.SourceInt)
	}
	if okT {
		setEnabled(target, link.TargetInt)
	}

	link.Enabled = req.Enabled
	if err := h.Repo.SaveLink(link); err != nil {
		h.Logger.Error("failed to persist link state", "id", link.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist link state"})
		return
	}

	h.Logger.Info("link toggled", "id", link.ID, "enabled", req.Enabled)
	h.BroadcastLinkToggled(link.ID, link.LabID, req.Enabled)

	c.JSON(http.StatusOK, link)
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
	if okS {
		h.hydrateNode(&source)
	}
	if okT {
		h.hydrateNode(&target)
	}

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
	if err := h.Repo.DeleteLink(id); err != nil {
		h.Logger.Error("failed to delete link from DB", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete link"})
		return
	}
	c.Status(http.StatusNoContent)
}
