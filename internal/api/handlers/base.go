package handlers

import (
	"log/slog"

	"open-veth/internal/config"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"
)

// Handler contains shared dependencies for all handlers
type Handler struct {
	Manager *orchestrator.Manager
	Network *orchestrator.NetworkManager
	Repo    storage.Repository
	Logger  *slog.Logger
	Config  *config.Config
}

// NewHandler creates a new handler with all dependencies
func NewHandler(mgr *orchestrator.Manager, repo storage.Repository, logger *slog.Logger, cfg *config.Config) *Handler {
	return &Handler{
		Manager: mgr,
		Network: orchestrator.NewNetworkManager(),
		Repo:    repo,
		Logger:  logger,
		Config:  cfg,
	}
}
