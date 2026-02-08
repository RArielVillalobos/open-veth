package handlers

import (
	"log/slog"

	"open-veth/internal/config"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"
)

// Handler contains shared dependencies for all handlers
type Handler struct {
	Manager *orchestrator.Manager
	Network *orchestrator.NetworkManager
	Repo    storage.Repository
	Runtime *RuntimeStore
	Logger  *slog.Logger
	Config  *config.Config
}

// NewHandler creates a new handler with all dependencies
func NewHandler(mgr *orchestrator.Manager, repo storage.Repository, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		Manager: mgr,
		Network: orchestrator.NewNetworkManager(),
		Repo:    repo,
		Runtime: NewRuntimeStore(),
		Logger:  logger,
		Config:  cfg,
	}
}

// hydrateNode populates runtime state (ContainerID, PID) from the in-memory RuntimeStore
func (h *Handler) hydrateNode(node *models.Node) {
	if state, ok := h.Runtime.Get(node.ID); ok {
		node.ContainerID = state.ContainerID
		node.PID = state.PID
	}
}

// hydrateNodes populates runtime state for a slice of nodes
func (h *Handler) hydrateNodes(nodes []models.Node) {
	for i := range nodes {
		h.hydrateNode(&nodes[i])
	}
}
