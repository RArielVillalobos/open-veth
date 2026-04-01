package handlers

import (
	"net/http"
	"testing"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupSnapshotRouter(h *Handler) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	api.POST("/laboratories/:id/save-state", h.SaveLabState)
	api.DELETE("/nodes/:id", h.DeleteNode)
	api.DELETE("/cleanup", h.HandleCleanup)
	return r
}

// --- SaveLabState ---

func TestSaveLabState_LabNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupSnapshotRouter(h)

	rec := doRequest(r, "POST", "/api/v1/laboratories/nonexistent/save-state", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSaveLabState_NoRunningContainers(t *testing.T) {
	h, repo := newTestHandler()
	_ = repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	// Node has no ContainerID (Manager is nil, not hydrated)
	_ = repo.SaveNode(models.Node{ID: "node-1", Name: "R1", Type: models.ROUTER, LabID: "lab-1"})

	r := setupSnapshotRouter(h)
	rec := doRequest(r, "POST", "/api/v1/laboratories/lab-1/save-state", nil)

	// No running containers → conflict
	assert.Equal(t, http.StatusConflict, rec.Code)
}

// --- DeleteNode with SnapshotImage ---

func TestDeleteNode_WithSnapshot_Manager_Nil(t *testing.T) {
	h, repo := newTestHandler()
	node := models.Node{
		ID:            "node-snap",
		Name:          "SERVER-1",
		Type:          models.SERVER,
		LabID:         "lab-1",
		SnapshotImage: models.SnapshotImageName("node-snap"),
	}
	_ = repo.SaveNode(node)

	r := setupSnapshotRouter(h)
	// Manager is nil → snapshot removal is skipped, node is still deleted from DB
	rec := doRequest(r, "DELETE", "/api/v1/nodes/node-snap", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, found := repo.GetNode("node-snap")
	assert.False(t, found)
}

func TestDeleteNode_WithSnapshot_DeletesFromDB(t *testing.T) {
	h, repo := newTestHandler()
	_ = repo.SaveNode(models.Node{
		ID:            "node-2",
		Name:          "LINUX-1",
		Type:          models.LINUX,
		LabID:         "lab-1",
		SnapshotImage: models.SnapshotImageName("node-2"),
	})

	r := setupSnapshotRouter(h)
	rec := doRequest(r, "DELETE", "/api/v1/nodes/node-2", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, found := repo.GetNode("node-2")
	assert.False(t, found)
}

func TestDeleteNode_WithoutSnapshot(t *testing.T) {
	h, repo := newTestHandler()
	_ = repo.SaveNode(models.Node{ID: "node-3", Name: "HOST-1", Type: models.HOST, LabID: "lab-1"})

	r := setupSnapshotRouter(h)
	rec := doRequest(r, "DELETE", "/api/v1/nodes/node-3", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, found := repo.GetNode("node-3")
	assert.False(t, found)
}

// --- HandleCleanup (nuke) ---

func TestHandleCleanup_ClearsDB(t *testing.T) {
	h, repo := newTestHandler()
	_ = repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	_ = repo.SaveNode(models.Node{ID: "node-1", Name: "R1", Type: models.ROUTER, LabID: "lab-1"})
	_ = repo.SaveNode(models.Node{
		ID:            "node-2",
		Name:          "SERVER-1",
		Type:          models.SERVER,
		LabID:         "lab-1",
		SnapshotImage: models.SnapshotImageName("node-2"),
	})

	// Manager is nil — Docker calls are skipped, DB is cleared
	r := setupSnapshotRouter(h)
	rec := doRequest(r, "DELETE", "/api/v1/cleanup", nil)

	assert.Equal(t, http.StatusOK, rec.Code)

	// DB should be empty after nuke
	nodes, _ := repo.ListNodesByLab("lab-1")
	assert.Empty(t, nodes)
	labs, _ := repo.ListLaboratories()
	assert.Empty(t, labs)
}
