package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"open-veth/internal/models"
)

// --- HTTP handler test ---

func TestGetDomainsHandler(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Setup lab with a simple topology: pc1 -- sw1 -- pc2
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	repo.SaveNode(models.Node{ID: "pc1", LabID: "lab-1", Type: models.HOST})
	repo.SaveNode(models.Node{ID: "pc2", LabID: "lab-1", Type: models.HOST})
	repo.SaveNode(models.Node{ID: "sw1", LabID: "lab-1", Type: models.SWITCH})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "pc1", TargetID: "sw1"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1", SourceID: "pc2", TargetID: "sw1"})

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-1/domains", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp domainsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.BroadcastDomains) != 1 {
		t.Errorf("expected 1 broadcast domain, got %d", len(resp.BroadcastDomains))
	}
	if len(resp.CollisionDomains) != 2 {
		t.Errorf("expected 2 collision domains, got %d", len(resp.CollisionDomains))
	}
}

func TestGetDomainsHandlerNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-999/domains", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing lab, got %d", w.Code)
	}
}

func TestGetDomainsHandlerEmptyLab(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Empty Lab"})

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-1/domains", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp domainsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.BroadcastDomains) != 0 {
		t.Errorf("expected 0 broadcast domains, got %d", len(resp.BroadcastDomains))
	}
	if len(resp.CollisionDomains) != 0 {
		t.Errorf("expected 0 collision domains, got %d", len(resp.CollisionDomains))
	}
}
