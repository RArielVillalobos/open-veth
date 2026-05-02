package handlers

import (
	"context"
	"testing"

	"open-veth/internal/models"

	"github.com/stretchr/testify/assert"
)

// TestStoreServicePorts_SkipsNonServiceNodes verifies that storeServicePorts
// returns immediately for types that don't expose service ports, without
// touching the RuntimeStore or the nil Manager.
func TestStoreServicePorts_SkipsNonServiceNodes(t *testing.T) {
	h, _ := newTestHandler() // Manager is nil — any call to it would panic

	skip := []models.NodeType{models.HOST, models.ROUTER, models.SWITCH, models.HUB, models.CLOUD, models.SERVER, models.TESTER}
	for _, typ := range skip {
		node := &models.Node{ID: "n1", Type: typ}
		h.storeServicePorts(context.Background(), node, "container-abc")
		_, stored := h.Runtime.Get("n1")
		assert.False(t, stored, "storeServicePorts should not write to RuntimeStore for type %q", typ)
	}
}

// TestRuntimeStore_HaproxyStatsPorts verifies that SetServicePorts stores the
// "stats" key and that it survives a subsequent Get.
func TestRuntimeStore_HaproxyStatsPorts(t *testing.T) {
	r := NewRuntimeStore()
	r.SetServicePorts("lb-1", map[string]int{"stats": 49234})

	state, ok := r.Get("lb-1")
	assert.True(t, ok)
	assert.Equal(t, 49234, state.ServicePorts["stats"])
}

// TestHydrateNode_HaproxyServicePorts verifies the full pipeline:
// storeServicePorts writes {"stats": port} → hydrateNode reads → Node.ServicePorts is populated.
// This is what the frontend consumes to render the "Open Stats" link.
func TestHydrateNode_HaproxyServicePorts(t *testing.T) {
	h, repo := newTestHandler()
	node := models.Node{ID: "lb-1", Name: "LB1", Type: models.HAPROXY, LabID: "lab-1"}
	_ = repo.SaveNode(node)

	h.Runtime.Set("lb-1", "container-xyz", 999)
	h.Runtime.SetServicePorts("lb-1", map[string]int{"stats": 49234})

	h.hydrateNode(&node)

	assert.Equal(t, "container-xyz", node.ContainerID)
	assert.Equal(t, 49234, node.ServicePorts["stats"])
}

// TestHydrateNode_HaproxyNoPortsYet verifies that ServicePorts is empty when
// the container is running but HAProxy has not yet reported its stats port.
// The frontend shows "Stats — starting..." in this state.
func TestHydrateNode_HaproxyNoPortsYet(t *testing.T) {
	h, repo := newTestHandler()
	node := models.Node{ID: "lb-1", Name: "LB1", Type: models.HAPROXY, LabID: "lab-1"}
	_ = repo.SaveNode(node)

	h.Runtime.Set("lb-1", "container-xyz", 999) // container up, no ports yet

	h.hydrateNode(&node)

	assert.Equal(t, "container-xyz", node.ContainerID)
	assert.Empty(t, node.ServicePorts)
}
