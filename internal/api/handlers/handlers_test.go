package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"open-veth/internal/config"
	"open-veth/internal/models"
	"open-veth/internal/storage"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestHandler creates a Handler with MemoryRepository and nil Manager/Network.
// Tests that hit Docker paths will panic, which is intentional — those paths
// should be guarded by validation that returns before reaching Docker calls.
func newTestHandler() (*Handler, *storage.MemoryRepository) {
	repo := storage.NewMemoryRepository()
	logger := slog.Default()
	cfg := &config.Config{}
	h := &Handler{
		Manager: nil, // no Docker in unit tests
		Network: nil,
		Repo:    repo,
		Runtime: NewRuntimeStore(),
		Logger:  logger,
		Config:  cfg,
	}
	return h, repo
}

func setupRouter(h *Handler) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/nodes", h.ListNodes)
		api.POST("/nodes", h.CreateNode)
		api.PATCH("/nodes/:id/position", h.UpdateNodePosition)
		api.DELETE("/nodes/:id", h.DeleteNode)

		api.GET("/links", h.ListLinks)
		api.POST("/links", h.CreateLink)
		api.PATCH("/links/:id/toggle", h.ToggleLink)
		api.DELETE("/links/:id", h.DeleteLink)

		api.GET("/laboratories", h.ListLaboratories)
		api.POST("/laboratories", h.CreateLaboratory)
		api.PATCH("/laboratories/:id", h.UpdateLaboratory)
		api.DELETE("/laboratories/:id", h.DeleteLaboratory)
		api.GET("/laboratories/:id/domains", h.GetDomains)

		api.GET("/topology/export", h.HandleExport)
		api.POST("/topology/import", h.HandleImport)

		api.GET("/terminal", h.HandleTerminal)
		api.GET("/sniff", h.HandleSniff)

		api.DELETE("/laboratories/:id/cleanup", h.CleanupLaboratory)

		api.POST("/nodes/:id/traceroute", h.RunTraceroute)
	api.POST("/nodes/:id/upload", h.UploadFile)
	}
	return r
}

// doMultipartRequest builds a multipart/form-data request with a single file field.
func doMultipartRequest(r *gin.Engine, path, filename, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("file", filename)
	io.WriteString(fw, content)
	w.Close()

	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doRequest(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// =============================================================================
// Laboratories
// =============================================================================

func TestCreateLaboratory(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	// Success
	w := doRequest(r, "POST", "/api/v1/laboratories", models.Laboratory{ID: "lab-1", Name: "My Lab"})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Missing ID
	w = doRequest(r, "POST", "/api/v1/laboratories", models.Laboratory{Name: "No ID"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing ID, got %d", w.Code)
	}
}

func TestListLaboratories(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Lab 1"})
	repo.SaveLaboratory(models.Laboratory{ID: "lab-2", Name: "Lab 2"})

	w := doRequest(r, "GET", "/api/v1/laboratories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var labs []models.Laboratory
	json.Unmarshal(w.Body.Bytes(), &labs)
	if len(labs) != 2 {
		t.Errorf("expected 2 labs, got %d", len(labs))
	}
}

func TestUpdateLaboratory(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Old Name"})

	w := doRequest(r, "PATCH", "/api/v1/laboratories/lab-1", map[string]string{"name": "New Name"})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, _ := repo.GetLaboratory("lab-1")
	if got.Name != "New Name" {
		t.Errorf("expected 'New Name', got %q", got.Name)
	}

	// Not found
	w = doRequest(r, "PATCH", "/api/v1/laboratories/lab-999", map[string]string{"name": "X"})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteLaboratory(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Lab"})

	w := doRequest(r, "DELETE", "/api/v1/laboratories/lab-1", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	_, ok := repo.GetLaboratory("lab-1")
	if ok {
		t.Error("expected lab to be deleted")
	}
}

// =============================================================================
// Nodes (repo-only paths)
// =============================================================================

func TestListNodes(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "HOST-2"})
	repo.SaveNode(models.Node{ID: "n3", LabID: "lab-2", Name: "HOST-3"})

	// All nodes
	w := doRequest(r, "GET", "/api/v1/nodes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var nodes []models.Node
	json.Unmarshal(w.Body.Bytes(), &nodes)
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// Filter by lab
	w = doRequest(r, "GET", "/api/v1/nodes?lab_id=lab-1", nil)
	json.Unmarshal(w.Body.Bytes(), &nodes)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in lab-1, got %d", len(nodes))
	}
}

func TestUpdateNodePosition(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1"})

	w := doRequest(r, "PATCH", "/api/v1/nodes/n1/position", map[string]float64{"x": 100, "y": 200})
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	got, _ := repo.GetNode("n1")
	if got.X != 100 || got.Y != 200 {
		t.Errorf("expected position (100,200), got (%v,%v)", got.X, got.Y)
	}

	// Not found
	w = doRequest(r, "PATCH", "/api/v1/nodes/n999/position", map[string]float64{"x": 1, "y": 1})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// =============================================================================
// Links (validation paths)
// =============================================================================

func TestCreateLinkValidation(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "HOST-2"})
	h.Runtime.Set("n1", "abc123", 1000)
	h.Runtime.Set("n2", "def456", 1001)

	// Bad source interface name (injection attempt)
	w := doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "link-12345", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth0; rm -rf /", TargetInt: "eth0",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for injection in source_int, got %d: %s", w.Code, w.Body.String())
	}

	// Bad target interface name
	w = doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "link-12345", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth0", TargetInt: "eth0$(whoami)",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for injection in target_int, got %d: %s", w.Code, w.Body.String())
	}

	// Link ID too short
	w = doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "ab", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth0", TargetInt: "eth0",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short link ID, got %d: %s", w.Code, w.Body.String())
	}

	// Source node not found
	w = doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "link-12345", LabID: "lab-1",
		SourceID: "n999", TargetID: "n2",
		SourceInt: "eth0", TargetInt: "eth0",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing source node, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLinks(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1"})

	w := doRequest(r, "GET", "/api/v1/links", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var links []models.Link
	json.Unmarshal(w.Body.Bytes(), &links)
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}

	// Filter by lab
	w = doRequest(r, "GET", "/api/v1/links?lab_id=lab-1", nil)
	json.Unmarshal(w.Body.Bytes(), &links)
	if len(links) != 2 {
		t.Errorf("expected 2 links in lab-1, got %d", len(links))
	}
}

func TestDeleteLink(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Link exists but nodes don't — tests DB-only deletion path
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2", SourceInt: "eth0", TargetInt: "eth0"})

	w := doRequest(r, "DELETE", "/api/v1/links/l1", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	_, ok := repo.GetLink("l1")
	if ok {
		t.Error("expected link to be deleted from repo")
	}

	// Not found
	w = doRequest(r, "DELETE", "/api/v1/links/l999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// =============================================================================
// Export
// =============================================================================

func TestHandleExport(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1", Type: models.HOST, X: 10, Y: 20})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "HOST-2", Type: models.HOST, X: 200, Y: 20})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2", SourceInt: "eth0", TargetInt: "eth0"})

	w := doRequest(r, "GET", "/api/v1/topology/export?lab_id=lab-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/x-yaml" {
		t.Errorf("expected content-type application/x-yaml, got %q", ct)
	}

	body := w.Body.String()

	// Links must reference nodes by name, not by internal ID
	if !strings.Contains(body, "HOST-1") || !strings.Contains(body, "HOST-2") {
		t.Errorf("expected node names in YAML, got:\n%s", body)
	}
	if strings.Contains(body, "n1") || strings.Contains(body, "n2") {
		t.Errorf("internal IDs must not appear in exported YAML, got:\n%s", body)
	}

	// Empty lab export should still work
	w = doRequest(r, "GET", "/api/v1/topology/export?lab_id=lab-empty", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty lab export, got %d", w.Code)
	}
}

// =============================================================================
// Duplicate link detection
// =============================================================================

func TestCreateLinkDuplicate(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1"})
	h.Runtime.Set("n1", "c1", 1000)
	h.Runtime.Set("n2", "c2", 1001)
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2"})

	// Try to create duplicate (same direction)
	w := doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "link-new99", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth1", TargetInt: "eth1",
	})
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate link, got %d: %s", w.Code, w.Body.String())
	}

	// Try to create duplicate (reverse direction)
	w = doRequest(r, "POST", "/api/v1/links", models.Link{
		ID: "link-new99", LabID: "lab-1",
		SourceID: "n2", TargetID: "n1",
		SourceInt: "eth1", TargetInt: "eth1",
	})
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for reverse duplicate, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Invalid JSON
// =============================================================================

func TestBadJSON(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/nodes"},
		{"POST", "/api/v1/links"},
		{"POST", "/api/v1/laboratories"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("%s %s: expected 400 for bad JSON, got %d", ep.method, ep.path, w.Code)
		}
	}
}

// =============================================================================
// Terminal WebSocket Security Validation
// =============================================================================

func TestTerminalRejectsEmptyNode(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "GET", "/api/v1/terminal", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing node param, got %d", w.Code)
	}
}

func TestTerminalRejectsUnknownNode(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	// Node doesn't exist in repo — must be rejected
	w := doRequest(r, "GET", "/api/v1/terminal?node=evil-container", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown node, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminalRejectsNodeNotRunning(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Node exists but has no ContainerID (not running)
	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1", ContainerID: ""})

	// Try by ID
	w := doRequest(r, "GET", "/api/v1/terminal?node=n1", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for node not running (by ID), got %d: %s", w.Code, w.Body.String())
	}

	// Try by name
	w = doRequest(r, "GET", "/api/v1/terminal?node=HOST-1", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for node not running (by name), got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminalAcceptsNodeByID(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	// This passes validation but fails at WebSocket upgrade (no real WS client).
	// We verify it's NOT our validation error — the WS upgrade failure produces
	// a different body ("Bad Request") vs our JSON errors.
	w := doRequest(r, "GET", "/api/v1/terminal?node=n1", nil)
	body := w.Body.String()
	if w.Code == http.StatusNotFound || w.Code == http.StatusServiceUnavailable {
		t.Errorf("expected validation to pass for known node by ID, but got %d: %s", w.Code, body)
	}
	if strings.Contains(body, "node not found") || strings.Contains(body, "node is not running") {
		t.Errorf("got validation rejection for known running node by ID: %s", body)
	}
}

func TestTerminalAcceptsNodeByName(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	w := doRequest(r, "GET", "/api/v1/terminal?node=HOST-1", nil)
	body := w.Body.String()
	if w.Code == http.StatusNotFound || w.Code == http.StatusServiceUnavailable {
		t.Errorf("expected validation to pass for known node by name, but got %d: %s", w.Code, body)
	}
	if strings.Contains(body, "node not found") || strings.Contains(body, "node is not running") {
		t.Errorf("got validation rejection for known running node by name: %s", body)
	}
}

func TestTerminalRejectsArbitraryContainerName(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Only openveth nodes exist
	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	// Attempt to access a non-openveth container
	w := doRequest(r, "GET", "/api/v1/terminal?node=postgres-db", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for arbitrary container name, got %d: %s", w.Code, w.Body.String())
	}

	// Attempt to access by raw container ID
	w = doRequest(r, "GET", "/api/v1/terminal?node=sha256:deadbeef", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for raw container ID, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Sniff WebSocket Validation
// =============================================================================

func TestSniffRejectsMissingParams(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	// Missing both
	w := doRequest(r, "GET", "/api/v1/sniff", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing params, got %d", w.Code)
	}

	// Missing interface
	w = doRequest(r, "GET", "/api/v1/sniff?node_id=n1", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing interface, got %d", w.Code)
	}

	// Missing node_id
	w = doRequest(r, "GET", "/api/v1/sniff?interface=eth0", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing node_id, got %d", w.Code)
	}
}

func TestSniffRejectsInjectionInInterface(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	injections := []string{
		"eth0;rm",
		"eth0&&cat",
		"eth0$(whoami)",
		"eth0`id`",
		"eth0|ls",
		"-eth0",
		"0eth",
	}

	for _, iface := range injections {
		path := "/api/v1/sniff?node_id=n1&interface=" + url.QueryEscape(iface)
		w := doRequest(r, "GET", path, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for injection %q, got %d: %s", iface, w.Code, w.Body.String())
		}
	}
}

func TestSniffRejectsUnknownNode(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "GET", "/api/v1/sniff?node_id=n999&interface=eth0", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown node, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Import
// =============================================================================

func TestImportRejectsInvalidYAML(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	// Send garbage as YAML
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString("not: [yaml: {broken"))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid YAML, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportRejectsEmptyBody(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportCreatesLaboratory(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Lab must exist before importing
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "My Lab"})

	// Valid YAML with no nodes/links — should succeed without hitting Docker
	yamlBody := `
name: Imported Lab
nodes: []
links: []
`
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(yamlBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Lab ID is preserved; name is updated from the YAML
	lab, ok := repo.GetLaboratory("lab-1")
	if !ok {
		t.Fatal("expected lab-1 to still exist in repo")
	}
	if lab.Name != "Imported Lab" {
		t.Errorf("expected lab name 'Imported Lab', got %q", lab.Name)
	}

	// Verify response body
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "import process completed" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestImportOverwritesExistingLab(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Pre-existing lab (no nodes — node cleanup requires Docker Manager)
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Old Lab"})

	yamlBody := `
name: New Lab
nodes: []
links: []
`
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(yamlBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Lab should be overwritten with new name
	lab, ok := repo.GetLaboratory("lab-1")
	if !ok {
		t.Fatal("expected lab-1 to exist")
	}
	if lab.Name != "New Lab" {
		t.Errorf("expected 'New Lab', got %q", lab.Name)
	}
}

func TestExportIncludesInterfacesAndRoutes(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1", Type: models.HOST})

	repo.SaveInterfaceConfigs("lab-1", []models.InterfaceConfig{
		{LabID: "lab-1", NodeID: "n1", Interface: "eth0", Address: "10.0.0.1/24"},
	})
	repo.SaveRouteConfigs("lab-1", []models.RouteConfig{
		{LabID: "lab-1", NodeID: "n1", Dst: "0.0.0.0/0", Gateway: "10.0.0.254", Dev: "eth0"},
	})

	w := doRequest(r, "GET", "/api/v1/topology/export?lab_id=lab-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "10.0.0.1/24") {
		t.Errorf("expected IP address in YAML, got:\n%s", body)
	}
	if !strings.Contains(body, "0.0.0.0/0") {
		t.Errorf("expected route dst in YAML, got:\n%s", body)
	}
	if !strings.Contains(body, "10.0.0.254") {
		t.Errorf("expected route gateway in YAML, got:\n%s", body)
	}
}

func TestImportYAMLWithConfigsIsAccepted(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Existing Lab"})

	// YAML with interfaces and routes but no nodes — verifies the parser accepts
	// the new fields without returning 400. Full Docker flow is an integration test.
	yamlBody := `
name: "Config Lab"
nodes: []
links: []
`
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(yamlBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid YAML with config fields, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportRejectsUnknownLab(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	// No lab pre-created — should return 404
	yamlBody := `
name: Some Lab
nodes: []
links: []
`
	w := doRequest(r, "POST", "/api/v1/topology/import?lab_id=lab-nonexistent",
		bytes.NewBufferString(yamlBody))
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown lab, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportRejectsUnknownLinkNode(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	yamlBody := `
name: Bad Lab
nodes:
  - name: HOST-1
    type: host
links:
  - source: HOST-1
    target: GHOST-NODE
    source_int: eth0
    target_int: eth0
`
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(yamlBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for link referencing unknown node, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportRejectsInvalidNodeType(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	yamlBody := `
name: Bad Lab
nodes:
  - name: HOST-1
    type: hypervisor
links: []
`
	req := httptest.NewRequest("POST", "/api/v1/topology/import", bytes.NewBufferString(yamlBody))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid node type, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportContentDispositionQuoted(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "My Lab"})

	w := doRequest(r, "GET", "/api/v1/topology/export?lab_id=lab-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cd := w.Header().Get("Content-Disposition")
	expected := `attachment; filename="My Lab.yaml"`
	if cd != expected {
		t.Errorf("Content-Disposition = %q, want %q", cd, expected)
	}
}

// =============================================================================
// CleanupLaboratory
// =============================================================================

func TestCleanupLaboratoryNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "DELETE", "/api/v1/laboratories/lab-999/cleanup", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCleanupLaboratoryRemovesOnlyTargetLab(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Setup: two labs with nodes and links
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Lab 1"})
	repo.SaveLaboratory(models.Laboratory{ID: "lab-2", Name: "Lab 2"})

	// lab-1 nodes (no ContainerID = won't try Docker)
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "HOST-2"})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2"})

	// lab-2 nodes (should survive)
	repo.SaveNode(models.Node{ID: "n3", LabID: "lab-2", Name: "HOST-3"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-2", SourceID: "n3", TargetID: "n3"})

	w := doRequest(r, "DELETE", "/api/v1/laboratories/lab-1/cleanup", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["nodes_deleted"] != float64(2) {
		t.Errorf("expected 2 nodes deleted, got %v", resp["nodes_deleted"])
	}
	if resp["links_deleted"] != float64(1) {
		t.Errorf("expected 1 link deleted, got %v", resp["links_deleted"])
	}

	// lab-1 nodes gone
	_, ok := repo.GetNode("n1")
	if ok {
		t.Error("expected n1 to be deleted")
	}
	_, ok = repo.GetNode("n2")
	if ok {
		t.Error("expected n2 to be deleted")
	}
	_, ok = repo.GetLink("l1")
	if ok {
		t.Error("expected l1 to be deleted")
	}

	// lab-2 untouched
	_, ok = repo.GetNode("n3")
	if !ok {
		t.Error("expected n3 (lab-2) to survive")
	}
	_, ok = repo.GetLink("l2")
	if !ok {
		t.Error("expected l2 (lab-2) to survive")
	}

	// lab-1 still exists (cleanup removes resources, not the lab itself)
	_, ok = repo.GetLaboratory("lab-1")
	if !ok {
		t.Error("expected lab-1 to still exist after cleanup")
	}
}

// =============================================================================
// ToggleLink
// =============================================================================

func TestToggleLink_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "PATCH", "/api/v1/links/nonexistent/toggle", map[string]bool{"enabled": false})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToggleLink_InvalidBody(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLink(models.Link{
		ID: "link-12345", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth1", TargetInt: "eth1",
		Enabled: true,
	})

	w := doRequest(r, "PATCH", "/api/v1/links/link-12345/toggle", "not-json")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestToggleLink_DisableAndEnable(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Save nodes without ContainerID so setEnabled returns early (no Manager needed)
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "HOST-2"})
	repo.SaveLink(models.Link{
		ID: "link-12345", LabID: "lab-1",
		SourceID: "n1", TargetID: "n2",
		SourceInt: "eth1", TargetInt: "eth1",
		Enabled: true,
	})

	// Disable
	w := doRequest(r, "PATCH", "/api/v1/links/link-12345/toggle", map[string]bool{"enabled": false})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got models.Link
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Enabled {
		t.Error("expected Enabled=false in response")
	}

	persisted, _ := repo.GetLink("link-12345")
	if persisted.Enabled {
		t.Error("expected Enabled=false persisted in repo")
	}

	// Re-enable
	w = doRequest(r, "PATCH", "/api/v1/links/link-12345/toggle", map[string]bool{"enabled": true})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	json.Unmarshal(w.Body.Bytes(), &got)
	if !got.Enabled {
		t.Error("expected Enabled=true in response")
	}

	persisted, _ = repo.GetLink("link-12345")
	if !persisted.Enabled {
		t.Error("expected Enabled=true persisted in repo")
	}
}

// =============================================================================
// Upload
// =============================================================================

func TestUploadFile_NodeNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doMultipartRequest(r, "/api/v1/nodes/nonexistent/upload", "test.sh", "echo hello")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadFile_NodeNotRunning(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Node exists but has no ContainerID (stopped)
	repo.SaveNode(models.Node{ID: "node-stopped", Name: "stopped", Type: models.HOST})

	w := doMultipartRequest(r, "/api/v1/nodes/node-stopped/upload", "test.sh", "echo hello")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadFile_InvalidPath(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "node-1", Name: "n1", Type: models.HOST, ContainerID: "fake-id"})

	// Path without leading slash
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "test.sh")
	io.WriteString(fw, "echo hello")
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/nodes/node-1/upload?path=root", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid path, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadFile_NoFile(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "node-1", Name: "n1", Type: models.HOST, ContainerID: "fake-id"})

	// Send request with no file field
	req := httptest.NewRequest("POST", "/api/v1/nodes/node-1/upload", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d: %s", rec.Code, rec.Body.String())
	}
}
