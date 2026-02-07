package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"
)

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents the full health check response
type HealthResponse struct {
	Status     string                     `json:"status"`
	Version    string                     `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
}

// HealthCheck returns the API status with component health details
func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.Config.Docker.ExecTimeout)
	defer cancel()

	response := HealthResponse{
		Status:     "healthy",
		Version:    "0.4-realtime",
		Components: make(map[string]ComponentHealth),
	}

	// Check Docker connectivity
	dockerHealth := h.checkDocker(ctx)
	response.Components["docker"] = dockerHealth
	if dockerHealth.Status != "healthy" {
		response.Status = "degraded"
	}

	// Check Database connectivity
	dbHealth := h.checkDatabase()
	response.Components["database"] = dbHealth
	if dbHealth.Status != "healthy" {
		response.Status = "degraded"
	}

	// Determine HTTP status code
	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// checkDocker verifies Docker daemon connectivity
func (h *Handler) checkDocker(ctx context.Context) ComponentHealth {
	_, err := h.Manager.GetDockerClient().Ping(ctx)
	if err != nil {
		return ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}
	return ComponentHealth{Status: "healthy"}
}

// checkDatabase verifies database connectivity
func (h *Handler) checkDatabase() ComponentHealth {
	// Try to list laboratories as a simple connectivity test
	_, err := h.Repo.ListLaboratories()
	if err != nil {
		return ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}
	return ComponentHealth{Status: "healthy"}
}

// HandleCleanup eliminates all containers with label openveth=true
func (h *Handler) HandleCleanup(c *gin.Context) {
	ctx := c.Request.Context()

	h.Logger.Info("starting system cleanup")

	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err != nil {
		h.Logger.Error("failed to list containers for cleanup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
		return
	}

	cleaned := 0
	for _, ct := range containers {
		if err := h.Manager.DeleteNode(ctx, ct.ID); err != nil {
			h.Logger.Warn("failed to remove container during cleanup", "container", ct.ID[:12], "error", err)
		} else {
			cleaned++
		}
	}

	h.Repo.ClearAll()

	h.Logger.Info("cleanup completed", "containers_removed", cleaned)
	c.JSON(http.StatusOK, gin.H{"message": "cleanup complete", "containers_removed": cleaned})
}

// ReconcileState ensures Docker matches the Database state (The Janitor)
func (h *Handler) ReconcileState(ctx context.Context) error {
	h.Logger.Info("running startup reconciliation")

	// 1. Get all OpenVeth containers (The Reality)
	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err != nil {
		return err
	}

	// 2. Get all Nodes from DB (The Desired State)
	nodes, err := h.Repo.ListNodes()
	if err != nil {
		return err
	}

	// 3. Create a lookup map for valid container IDs
	validContainers := make(map[string]bool)
	for _, node := range nodes {
		validContainers["/"+node.Name] = true
		validContainers[node.Name] = true
	}

	// Build map of running containers by name for smart reconciliation
	runningContainers := make(map[string]types.Container)
	for _, ctr := range containers {
		for _, name := range ctr.Names {
			runningContainers[strings.TrimPrefix(name, "/")] = ctr
		}
	}

	// 4. Hunt for Zombies
	zombieCount := 0
	for _, ctr := range containers {
		isZombie := true
		for _, name := range ctr.Names {
			if validContainers[name] {
				isZombie = false
				break
			}
		}

		if isZombie {
			h.Logger.Warn("zombie container detected, terminating", "names", ctr.Names, "id", ctr.ID[:12])
			if err := h.Manager.DeleteNode(ctx, ctr.ID); err != nil {
				h.Logger.Error("failed to kill zombie", "id", ctr.ID[:12], "error", err)
			} else {
				zombieCount++
			}
		}
	}

	// 5. Revive Missing/Stopped Nodes (The Resurrection) - skip already running ones
	skipped, revived, failed := 0, 0, 0
	var failedNodes []string

	h.Logger.Info("reconciling nodes from database", "count", len(nodes))
	for _, node := range nodes {
		// Check if container is already running
		if ctr, running := runningContainers[node.Name]; running && ctr.State == "running" {
			pid, _ := h.Manager.GetNodePID(ctx, ctr.ID)
			node.ContainerID = ctr.ID
			node.PID = pid
			if err := h.Repo.SaveNode(node); err != nil {
				h.Logger.Error("failed to persist running node", "node", node.Name, "error", err)
			}
			skipped++
			h.Logger.Debug("node already running, skipped", "node", node.Name)
			continue
		}

		// Create missing node
		cid, err := h.Manager.CreateNode(ctx, node)
		if err != nil {
			h.Logger.Warn("failed to revive node", "node", node.Name, "error", err)
			failed++
			failedNodes = append(failedNodes, node.Name)
			continue
		}

		// Update DB with fresh PID
		pid, err := h.Manager.GetNodePID(ctx, cid)
		if err != nil {
			h.Logger.Warn("failed to get PID during reconciliation", "node", node.Name, "error", err)
		}
		node.ContainerID = cid
		node.PID = pid
		if err := h.Repo.SaveNode(node); err != nil {
			h.Logger.Error("failed to persist reconciled node", "node", node.Name, "error", err)
		}
		revived++
	}

	// 6. Restore Links
	linksRestored, linksFailed := 0, 0
	h.Logger.Info("restoring network links")
	links, err := h.Repo.ListLinks()
	if err != nil {
		h.Logger.Warn("failed to list links for restoration", "error", err)
	} else {
		for _, l := range links {
			src, okS := h.Repo.GetNode(l.SourceID)
			tgt, okT := h.Repo.GetNode(l.TargetID)

			if okS && okT && src.PID > 0 && tgt.PID > 0 {
				if err := h.Network.CreateLink(l, src.PID, tgt.PID); err != nil {
					h.Logger.Debug("link restoration note", "link", l.ID, "error", err)
					linksFailed++
				} else {
					linksRestored++
				}

				// Re-attach to bridges
				if src.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, src.ContainerID, l.SourceInt)
				}
				if tgt.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, tgt.ContainerID, l.TargetInt)
				}
			}
		}
	}

	// 7. Restore saved IP configurations
	h.Logger.Info("restoring saved IP configurations")
	labs, _ := h.Repo.ListLaboratories()
	restoredIPs := 0
	for _, lab := range labs {
		configs, err := h.Repo.GetInterfaceConfigsByLab(lab.ID)
		if err != nil {
			h.Logger.Warn("failed to fetch saved configs", "lab", lab.ID, "error", err)
			continue
		}
		for _, cfg := range configs {
			node, ok := h.Repo.GetNode(cfg.NodeID)
			if !ok || node.ContainerID == "" {
				continue
			}
			if err := h.Manager.ConfigureInterface(ctx, node.ContainerID, cfg.Interface, cfg.Address); err != nil {
				h.Logger.Warn("failed to restore IP config", "node", node.Name, "interface", cfg.Interface, "address", cfg.Address, "error", err)
			} else {
				restoredIPs++
			}
		}
	}

	// 8. Final summary
	if len(failedNodes) > 0 {
		h.Logger.Warn("nodes failed to revive", "nodes", failedNodes)
	}
	h.Logger.Info("reconciliation complete",
		"zombies_killed", zombieCount,
		"nodes_skipped", skipped,
		"nodes_revived", revived,
		"nodes_failed", failed,
		"links_restored", linksRestored,
		"links_failed", linksFailed,
		"ips_restored", restoredIPs,
	)

	return nil
}

// SaveAllLabsState captures and persists IP configuration for all laboratories.
// Called during graceful shutdown to preserve state without manual user action.
func (h *Handler) SaveAllLabsState(ctx context.Context) error {
	labs, err := h.Repo.ListLaboratories()
	if err != nil {
		return fmt.Errorf("failed to list laboratories: %w", err)
	}

	totalSaved := 0
	for _, lab := range labs {
		nodes, err := h.Repo.ListNodesByLab(lab.ID)
		if err != nil {
			h.Logger.Warn("failed to list nodes for save state", "lab", lab.ID, "error", err)
			continue
		}

		var configs []models.InterfaceConfig
		for _, node := range nodes {
			if node.ContainerID == "" {
				continue
			}

			ifaces, err := h.Manager.GetNodeInterfaces(ctx, node.ContainerID)
			if err != nil {
				h.Logger.Warn("failed to get interfaces for node", "node", node.Name, "error", err)
				continue
			}

			for _, iface := range ifaces {
				if iface.Name == "lo" || iface.Name == "mgmt0" {
					continue
				}

				for _, addr := range iface.IPAddresses {
					if len(addr.Address) > 4 && addr.Address[:4] == "fe80" {
						continue
					}
					configs = append(configs, models.InterfaceConfig{
						LabID:     lab.ID,
						NodeID:    node.ID,
						Interface: iface.Name,
						Address:   fmt.Sprintf("%s/%d", addr.Address, addr.Prefix),
					})
				}
			}
		}

		if err := h.Repo.SaveInterfaceConfigs(lab.ID, configs); err != nil {
			h.Logger.Warn("failed to save interface configs", "lab", lab.ID, "error", err)
			continue
		}
		totalSaved += len(configs)
	}

	h.Logger.Info("all labs state saved", "labs", len(labs), "configs_saved", totalSaved)
	return nil
}
