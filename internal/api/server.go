package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"open-veth/internal/api/handlers"
	"open-veth/internal/config"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server encapsulates the HTTP router and dependencies
type Server struct {
	router       *gin.Engine
	handler      *handlers.Handler
	logger       *slog.Logger
	config       *config.Config
	autoSaveStop chan struct{}
}

// NewServer creates and configures the API server instance
func NewServer(mgr *orchestrator.Manager, cfg *config.Config, logger *slog.Logger) *Server {
	r := gin.Default()

	// CORS configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("Authorization")
	r.Use(cors.New(corsConfig))

	// Initialize Repository using config
	var repo storage.Repository
	dbRepo, err := storage.NewGormRepository(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		logger.Warn("failed to initialize database, falling back to memory",
			"driver", cfg.Database.Driver, "error", err)
		repo = storage.NewMemoryRepository()
	} else {
		repo = dbRepo
		logger.Info("database initialized", "driver", cfg.Database.Driver)
	}

	// Create handler with all dependencies
	h := handlers.NewHandler(mgr, repo, logger, cfg)

	s := &Server{
		router:  r,
		handler: h,
		logger:  logger,
		config:  cfg,
	}

	s.setupRoutes()
	return s
}

// setupRoutes defines API endpoints
func (s *Server) setupRoutes() {
	// Health Check
	s.router.GET("/health", s.handler.HealthCheck)

	api := s.router.Group("/api/v1")
	{
		// WebSocket endpoints
		api.GET("/terminal", s.handler.HandleTerminal)
		api.GET("/sniff", s.handler.HandleSniff)
		api.GET("/events", s.handler.HandleNetworkEvents)

		// Nodes
		api.GET("/nodes", s.handler.ListNodes)
		api.POST("/nodes", s.handler.CreateNode)
		api.PATCH("/nodes/:id/position", s.handler.UpdateNodePosition)
		api.DELETE("/nodes/:id", s.handler.DeleteNode)
		api.GET("/nodes/:id/interfaces", s.handler.GetNodeInterfaces)
		api.GET("/nodes/:id/routes", s.handler.GetNodeRoutes)

		// Links
		api.GET("/links", s.handler.ListLinks)
		api.POST("/links", s.handler.CreateLink)
		api.DELETE("/links/:id", s.handler.DeleteLink)

		// Laboratories
		api.GET("/laboratories", s.handler.ListLaboratories)
		api.POST("/laboratories", s.handler.CreateLaboratory)
		api.POST("/laboratories/:id/activate", s.handler.ActivateLaboratory)
		api.POST("/laboratories/:id/save-state", s.handler.SaveLabState)
		api.DELETE("/laboratories/:id/cleanup", s.handler.CleanupLaboratory)
		api.PATCH("/laboratories/:id", s.handler.UpdateLaboratory)
		api.DELETE("/laboratories/:id", s.handler.DeleteLaboratory)

		// Topology Export/Import
		api.GET("/topology/export", s.handler.HandleExport)
		api.POST("/topology/import", s.handler.HandleImport)

		// Global Cleanup
		api.DELETE("/system/cleanup", s.handler.HandleCleanup)
	}
}

// Handler returns the underlying http.Handler for use with custom http.Server
func (s *Server) Handler() http.Handler {
	return s.router
}

// Logger returns the server's logger
func (s *Server) Logger() *slog.Logger {
	return s.logger
}

// Reconcile performs startup state reconciliation
func (s *Server) Reconcile(ctx context.Context) error {
	return s.handler.ReconcileState(ctx)
}

// SaveState saves the IP configuration of all laboratories before shutdown
func (s *Server) SaveState(ctx context.Context) error {
	return s.handler.SaveAllLabsState(ctx)
}

// StartAutoSave begins periodic IP configuration saving
func (s *Server) StartAutoSave(interval time.Duration) {
	s.autoSaveStop = make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		s.logger.Info("auto-save started", "interval", interval.String())
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := s.handler.SaveAllLabsState(ctx); err != nil {
					s.logger.Warn("auto-save failed", "error", err)
				} else {
					s.logger.Debug("auto-save completed")
				}
				cancel()
			case <-s.autoSaveStop:
				ticker.Stop()
				s.logger.Info("auto-save stopped")
				return
			}
		}
	}()
}

// StopAutoSave signals the auto-save goroutine to stop
func (s *Server) StopAutoSave() {
	if s.autoSaveStop != nil {
		close(s.autoSaveStop)
	}
}

// StartEventHub starts the network events hub in a background goroutine
func (s *Server) StartEventHub() {
	if s.handler.EventHub != nil {
		s.logger.Info("starting network events hub")
		go s.handler.EventHub.Run()
	}
}

// StopEventHub signals the network events hub to shut down
func (s *Server) StopEventHub() {
	if s.handler.EventHub != nil {
		s.handler.EventHub.Stop()
	}
}

// Run starts the HTTP server (simple mode, no graceful shutdown)
func (s *Server) Run(addr string) error {
	s.logger.Info("starting OpenVeth API server", "address", addr)

	ctx := context.Background()
	if err := s.handler.ReconcileState(ctx); err != nil {
		s.logger.Warn("reconciliation failed", "error", err)
	}

	return s.router.Run(addr)
}
