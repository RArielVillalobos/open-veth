package handlers

import (
	"net/http"
	"testing"

	"open-veth/internal/models"
)

func TestGetNodeMacTable_NodeNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "GET", "/api/v1/nodes/n999/mactable", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetNodeMacTable_NodeNotRunning(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "SW-1", Type: models.SWITCH})

	w := doRequest(r, "GET", "/api/v1/nodes/n1/mactable", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}
