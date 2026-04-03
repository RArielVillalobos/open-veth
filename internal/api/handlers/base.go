package handlers

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"open-veth/internal/config"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"
)

// Handler contains shared dependencies for all handlers
type Handler struct {
	Manager  *orchestrator.Manager
	Network  *orchestrator.NetworkManager
	Repo     storage.Repository
	Runtime  *RuntimeStore
	Logger   *slog.Logger
	Config   *config.Config
	EventHub *NetworkEventHub

	// labOpMu serializes operations that save state, destroy containers,
	// or rebuild labs. Prevents the auto-save goroutine from reading
	// stale containers while ActivateLaboratory is mid-nuke.
	labOpMu sync.Mutex
}

// NewHandler creates a new handler with all dependencies
func NewHandler(mgr *orchestrator.Manager, repo storage.Repository, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		Manager:  mgr,
		Network:  orchestrator.NewNetworkManager(),
		Repo:     repo,
		Runtime:  NewRuntimeStore(),
		Logger:   logger,
		Config:   cfg,
		EventHub: NewNetworkEventHub(),
	}
}

// hydrateNode populates runtime state (ContainerID, PID, ServicePorts) from the in-memory RuntimeStore
func (h *Handler) hydrateNode(node *models.Node) {
	if state, ok := h.Runtime.Get(node.ID); ok {
		node.ContainerID = state.ContainerID
		node.PID = state.PID
		if len(state.ServicePorts) > 0 {
			node.ServicePorts = state.ServicePorts
		}
	}
}

// hydrateNodes populates runtime state for a slice of nodes
func (h *Handler) hydrateNodes(nodes []models.Node) {
	for i := range nodes {
		h.hydrateNode(&nodes[i])
	}
}

// storeMonitorPorts retrieves the dynamically assigned host ports for a MONITOR node.
// It stores Prometheus immediately (starts fast) and waits asynchronously for Grafana
// to be ready before storing its port and broadcasting node:updated.
func (h *Handler) storeMonitorPorts(ctx context.Context, node *models.Node, containerID string) {
	if node.Type != models.MONITOR {
		return
	}
	ports, err := h.Manager.GetServicePorts(ctx, containerID)
	if err != nil {
		h.Logger.Warn("failed to get monitor ports", "node", node.Name, "error", err)
		return
	}

	// Store Prometheus port immediately so the frontend shows it right away
	promOnly := map[string]int{}
	if p, ok := ports["prometheus"]; ok {
		promOnly["prometheus"] = p
	}
	h.Runtime.SetServicePorts(node.ID, promOnly)
	node.ServicePorts = promOnly

	// Wait for Grafana asynchronously — it takes 30-60s with systemd
	nodeID := node.ID
	nodeName := node.Name
	labID := node.LabID
	grafanaPort, hasGrafana := ports["grafana"]
	if !hasGrafana {
		return
	}
	go func() {
		h.Logger.Info("waiting for grafana to be ready", "node", nodeName)
		if err := h.Manager.WaitForHTTP(context.Background(), containerID, 3000, 3*time.Minute); err != nil {
			h.Logger.Warn("grafana did not become ready", "node", nodeName, "error", err)
			return
		}
		h.Logger.Info("grafana ready", "node", nodeName, "port", grafanaPort)
		current, _ := h.Runtime.Get(nodeID)
		merged := map[string]int{}
		for k, v := range current.ServicePorts {
			merged[k] = v
		}
		merged["grafana"] = grafanaPort
		h.Runtime.SetServicePorts(nodeID, merged)
		h.EventHub.Broadcast(NetworkEvent{
			Type:      "node:updated",
			NodeID:    nodeID,
			LabID:     labID,
			Timestamp: time.Now().Unix(),
		})
	}()
}
