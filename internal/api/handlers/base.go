package handlers

import (
	"context"
	"log/slog"
	"sync"

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

// storeMonitorPorts retrieves and stores the dynamically assigned host ports for a MONITOR node.
func (h *Handler) storeMonitorPorts(ctx context.Context, node *models.Node, containerID string) {
	if node.Type != models.MONITOR {
		return
	}
	ports, err := h.Manager.GetServicePorts(ctx, containerID)
	if err != nil {
		h.Logger.Warn("failed to get monitor ports", "node", node.Name, "error", err)
		return
	}
	h.Runtime.SetServicePorts(node.ID, ports)
	node.ServicePorts = ports
}
