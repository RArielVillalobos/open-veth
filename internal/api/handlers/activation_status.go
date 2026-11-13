package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type activationStatus struct {
	LabID       string `json:"lab_id"`
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
}

func (h *Handler) finishActivation(labID, status, reason string) {
	h.activationMu.Lock()
	defer h.activationMu.Unlock()
	if h.activation.LabID == labID {
		h.activation.Status = status
		h.activation.Reason = reason
	}
}

// GetLabActivation reports the latest activation result without starting another operation.
func (h *Handler) GetLabActivation(c *gin.Context) {
	h.activationMu.RLock()
	result := h.activation
	h.activationMu.RUnlock()
	if result.LabID != c.Param("id") {
		c.JSON(http.StatusOK, activationStatus{LabID: c.Param("id"), Status: "unknown"})
		return
	}
	c.JSON(http.StatusOK, result)
}
