package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
)

func TestDeleteLaboratoryCleansDockerBeforePersistence(t *testing.T) {
	for _, failure := range []string{"", "container", "snapshot"} {
		t.Run(failure, func(t *testing.T) {
			var containersRemoved, snapshotsRemoved atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/c1"):
					containersRemoved.Add(1)
					if failure == "container" {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]string{"message": "container removal failed"})
						return
					}
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/images/"):
					snapshotsRemoved.Add(1)
					if failure == "snapshot" {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]string{"message": "snapshot removal failed"})
						return
					}
					_, _ = w.Write([]byte("[]"))
				default:
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
			t.Setenv("DOCKER_API_VERSION", "1.45")
			h, repo := newTestHandler()
			manager, err := orchestrator.NewManager(h.Logger)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.GetDockerClient().Close()
			h.Manager = manager
			_ = repo.SaveLaboratory(models.Laboratory{ID: "lab"})
			_ = repo.SaveNode(models.Node{ID: "n1", Name: "router", LabID: "lab", SnapshotImage: "openveth-snapshot:n1"})
			h.Runtime.Set("n1", "c1", 123)
			response := doRequest(setupRouter(h), "DELETE", "/api/v1/laboratories/lab", nil)
			want := http.StatusNoContent
			if failure != "" {
				want = http.StatusInternalServerError
			}
			if response.Code != want {
				t.Fatalf("expected %d, got %d: %s", want, response.Code, response.Body.String())
			}
			_, labExists := repo.GetLaboratory("lab")
			_, nodeExists := repo.GetNode("n1")
			_, runtimeExists := h.Runtime.Get("n1")
			if labExists != (failure != "") || nodeExists != (failure != "") || runtimeExists != (failure != "") {
				t.Fatal("cleanup lost records needed for retry, or retained successfully deleted records")
			}
			wantSnapshots := int32(1)
			if failure == "container" {
				wantSnapshots = 0
			}
			if containersRemoved.Load() != 1 || snapshotsRemoved.Load() != wantSnapshots {
				t.Fatal("incorrect Docker cleanup sequence")
			}
		})
	}
}

func TestLabMutationsRejectDuringActivation(t *testing.T) {
	h, repo := newTestHandler()
	_ = repo.SaveLaboratory(models.Laboratory{ID: "lab"})
	r := setupRouter(h)
	h.labOpMu.Lock()
	defer h.labOpMu.Unlock()
	for _, endpoint := range []struct{ method, path string }{
		{"DELETE", "/api/v1/laboratories/lab"},
		{"DELETE", "/api/v1/laboratories/lab/cleanup"},
		{"DELETE", "/api/v1/system/cleanup"},
		{"POST", "/api/v1/laboratories/lab/save-state"},
		{"POST", "/api/v1/nodes"},
		{"POST", "/api/v1/links"},
	} {
		response := doRequest(r, endpoint.method, endpoint.path, nil)
		if response.Code != http.StatusConflict {
			t.Fatalf("%s %s: expected conflict, got %d", endpoint.method, endpoint.path, response.Code)
		}
	}
	if _, found := repo.GetLaboratory("lab"); !found {
		t.Fatal("busy lab was deleted")
	}
}

func TestCreateLinkRejectsInvalidOwnershipAndOccupiedInterface(t *testing.T) {
	for _, scenario := range []string{"cross-lab", "occupied-source", "occupied-target", "management"} {
		t.Run(scenario, func(t *testing.T) {
			h, repo := newTestHandler()
			_ = repo.SaveNode(models.Node{ID: "n1", LabID: "lab"})
			_ = repo.SaveNode(models.Node{ID: "n2", LabID: "lab"})
			link := models.Link{ID: "link-new", LabID: "lab", SourceID: "n1", TargetID: "n2", SourceInt: "eth0", TargetInt: "eth0"}
			want := http.StatusConflict
			switch scenario {
			case "cross-lab":
				_ = repo.SaveNode(models.Node{ID: "n2", LabID: "other"})
				want = http.StatusBadRequest
			case "occupied-source":
				_ = repo.SaveLink(models.Link{ID: "old", LabID: "lab", SourceID: "n3", TargetID: "n1", TargetInt: "eth0"})
			case "occupied-target":
				_ = repo.SaveLink(models.Link{ID: "old", LabID: "lab", SourceID: "n2", TargetID: "n3", SourceInt: "eth0"})
			case "management":
				link.SourceInt = "docker0"
				want = http.StatusBadRequest
			}
			response := doRequest(setupRouter(h), "POST", "/api/v1/links", link)
			if response.Code != want {
				t.Fatalf("expected %d, got %d: %s", want, response.Code, response.Body.String())
			}
		})
	}
}
