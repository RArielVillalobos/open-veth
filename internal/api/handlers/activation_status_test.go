package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestActivationStatus(t *testing.T) {
	for _, status := range []string{"activating", "succeeded", "failed"} {
		t.Run(status, func(t *testing.T) {
			h := &Handler{activation: activationStatus{LabID: "lab-1", OperationID: "op-1", Status: "activating"}}
			if status != "activating" {
				h.finishActivation("lab-1", status, "test reason")
			}
			router := gin.New()
			router.GET("/laboratories/:id/activation", h.GetLabActivation)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/laboratories/lab-1/activation", nil))
			var result activationStatus
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || result.Status != status || result.OperationID != "op-1" {
				t.Fatalf("unexpected response: %d %+v", response.Code, result)
			}
			response = httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/laboratories/other/activation", nil))
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "unknown" {
				t.Fatalf("expected unknown, got %s", result.Status)
			}
		})
	}
}

func TestActivationResultRecordedWithoutWebSocketClients(t *testing.T) {
	h := &Handler{activation: activationStatus{LabID: "lab-1", OperationID: "op-1", Status: "activating"}}
	h.BroadcastLabActivated("lab-1", 1, 1, 0)
	if h.activation.Status != "succeeded" {
		t.Fatal("success was not recorded")
	}
	h.BroadcastLabActivationFailed("other", "stale failure")
	if h.activation.Status != "succeeded" {
		t.Fatal("unrelated result changed the operation")
	}
	h.BroadcastLabActivationFailed("lab-1", "restore failed")
	if h.activation.Status != "failed" || h.activation.Reason != "restore failed" {
		t.Fatal("failure was not recorded")
	}
}
