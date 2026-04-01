package handlers

import (
	"net/http"
	"testing"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupPowerRouter(h *Handler) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	api.POST("/nodes/:id/stop", h.StopNode)
	api.POST("/nodes/:id/start", h.StartNode)
	return r
}

// --- StopNode ---

func TestStopNode_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupPowerRouter(h)

	rec := doRequest(r, "POST", "/api/v1/nodes/nonexistent/stop", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStopNode_NoContainer(t *testing.T) {
	h, repo := newTestHandler()
	node := models.Node{ID: "node-1", Name: "R1", Type: models.ROUTER, LabID: "lab-1", Status: models.NodeStatusRunning}
	_ = repo.SaveNode(node)
	// No entry in RuntimeStore → hydrateNode leaves ContainerID empty

	r := setupPowerRouter(h)
	rec := doRequest(r, "POST", "/api/v1/nodes/node-1/stop", nil)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// --- StartNode ---

func TestStartNode_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupPowerRouter(h)

	rec := doRequest(r, "POST", "/api/v1/nodes/nonexistent/start", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStartNode_NoContainer(t *testing.T) {
	h, repo := newTestHandler()
	node := models.Node{ID: "node-1", Name: "R1", Type: models.ROUTER, LabID: "lab-1", Status: models.NodeStatusStopped}
	_ = repo.SaveNode(node)

	r := setupPowerRouter(h)
	rec := doRequest(r, "POST", "/api/v1/nodes/node-1/start", nil)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
