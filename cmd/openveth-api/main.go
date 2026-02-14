package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"open-veth/internal/api"
	"open-veth/internal/config"
	"open-veth/internal/orchestrator"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	mgr, err := orchestrator.NewManager(logger)
	if err != nil {
		logger.Error("failed to create orchestrator", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := mgr.TestConnection(ctx); err != nil {
		logger.Error("docker connection failed", "error", err)
		os.Exit(1)
	}

	srv := api.NewServer(mgr, cfg, logger)
	srv.StartEventHub()
	srv.StartAutoSave(cfg.Server.AutoSaveInterval)

	httpServer := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: srv.Handler(),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("starting OpenVeth API server", "address", cfg.Server.Address)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down server...")

	srv.StopAutoSave()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.SaveState(shutdownCtx); err != nil {
		logger.Warn("failed to save state on shutdown", "error", err)
	}

	srv.StopEventHub()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
