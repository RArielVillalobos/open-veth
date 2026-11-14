package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type domainsResponse struct {
	BroadcastDomains []Domain `json:"broadcast_domains"`
	CollisionDomains []Domain `json:"collision_domains"`
}

// --- HTTP Handler ---

// GetDomains computes broadcast and collision domains for a laboratory.
func (h *Handler) GetDomains(c *gin.Context) {
	labID := c.Param("id")

	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for domains", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	links, err := h.Repo.ListLinksByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list links for domains", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	broadcast := computeBroadcastDomains(nodes, links)
	collision := computeCollisionDomains(nodes, links)

	if broadcast == nil {
		broadcast = []Domain{}
	}
	if collision == nil {
		collision = []Domain{}
	}

	c.JSON(http.StatusOK, domainsResponse{
		BroadcastDomains: broadcast,
		CollisionDomains: collision,
	})
}
