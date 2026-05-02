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

	cleaned := 0
	volumesRemoved := 0
	snapshotsRemoved := 0

	if h.Manager != nil {
		containers, err := h.Manager.GetOpenVethContainers(ctx)
		if err != nil {
			h.Logger.Error("failed to list containers for cleanup", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}
		for _, ct := range containers {
			if err := h.Manager.DeleteNode(ctx, ct.ID); err != nil {
				h.Logger.Warn("failed to remove container during cleanup", "container", ct.ID[:12], "error", err)
			} else {
				cleaned++
			}
		}
		volumesRemoved = h.Manager.RemoveAllOpenVethVolumes(ctx)
		snapshotsRemoved = h.Manager.RemoveAllSnapshotImages(ctx)
	}

	h.Repo.ClearAll()
	h.Runtime.Clear()

	h.Logger.Info("cleanup completed", "containers_removed", cleaned, "volumes_removed", volumesRemoved, "snapshots_removed", snapshotsRemoved)
	c.JSON(http.StatusOK, gin.H{"message": "cleanup complete", "containers_removed": cleaned, "volumes_removed": volumesRemoved, "snapshots_removed": snapshotsRemoved})
}

// ReconcileState ensures Docker matches the Database state (The Janitor)
func (h *Handler) ReconcileState(ctx context.Context) error {
	h.reconciling.Store(true)
	defer h.reconciling.Store(false)

	h.labOpMu.Lock()
	defer h.labOpMu.Unlock()

	h.Logger.Info("running startup reconciliation")

	// 1. Gather state from Docker and DB
	containers, err := h.Manager.GetOpenVethContainers(ctx)
	if err != nil {
		return err
	}
	nodes, err := h.Repo.ListNodes()
	if err != nil {
		return err
	}

	// 2. Build lookup maps
	knownNames := make(map[string]bool, len(nodes)*2)
	for _, n := range nodes {
		knownNames[n.Name] = true
		knownNames["/"+n.Name] = true
	}

	containerByName := make(map[string]types.Container, len(containers))
	for _, ctr := range containers {
		for _, name := range ctr.Names {
			containerByName[strings.TrimPrefix(name, "/")] = ctr
		}
	}

	// 3. Reconcile
	zombies := h.killZombies(ctx, containers, knownNames)
	nodeMap, skipped, revived, failed := h.reconcileNodes(ctx, nodes, containerByName)
	linksOK, linksFail := h.restoreLinks(ctx, nodeMap)
	restoredIPs, restoredRoutes := h.restoreIPsAndRoutes(ctx, nodeMap)

	// 4. Summary
	h.Logger.Info("reconciliation complete",
		"zombies_killed", zombies,
		"nodes_skipped", skipped,
		"nodes_revived", revived,
		"nodes_failed", failed,
		"links_restored", linksOK,
		"links_failed", linksFail,
		"ips_restored", restoredIPs,
		"routes_restored", restoredRoutes,
	)
	return nil
}

// killZombies removes containers that exist in Docker but not in the DB
func (h *Handler) killZombies(ctx context.Context, containers []types.Container, knownNames map[string]bool) int {
	killed := 0
	for _, ctr := range containers {
		isZombie := true
		for _, name := range ctr.Names {
			if knownNames[name] {
				isZombie = false
				break
			}
		}
		if isZombie {
			h.Logger.Warn("zombie container detected", "names", ctr.Names, "id", ctr.ID[:12])
			if err := h.Manager.DeleteNode(ctx, ctr.ID); err != nil {
				h.Logger.Error("failed to kill zombie", "id", ctr.ID[:12], "error", err)
			} else {
				killed++
			}
		}
	}
	return killed
}

// reconcileNodes ensures every DB node has a running container.
// Returns a nodeMap with hydrated runtime state for use by restoreLinks/restoreIPs.
func (h *Handler) reconcileNodes(ctx context.Context, nodes []models.Node, containerByName map[string]types.Container) (nodeMap map[string]models.Node, skipped, revived, failed int) {
	nodeMap = make(map[string]models.Node, len(nodes))

	for _, node := range nodes {
		// Already running → just capture runtime state
		if ctr, ok := containerByName[node.Name]; ok && ctr.State == "running" {
			pid, err := h.Manager.GetNodePID(ctx, ctr.ID)
			if err != nil {
				h.Logger.Warn("failed to get PID for running node", "node", node.Name, "error", err)
			}
			h.Runtime.Set(node.ID, ctr.ID, pid)
			h.storeServicePorts(ctx, &node, ctr.ID)
			node.ContainerID = ctr.ID
			node.PID = pid
			nodeMap[node.ID] = node
			skipped++
			continue
		}

		// Exists but stopped (e.g. Docker Desktop restart, node was powered off) → start it
		if ctr, ok := containerByName[node.Name]; ok && ctr.State != "running" {
			h.Logger.Info("restarting stopped container", "node", node.Name, "state", ctr.State)
			if err := h.Manager.StartNode(ctx, ctr.ID); err != nil {
				h.Logger.Warn("failed to restart stopped container, will recreate", "node", node.Name, "error", err)
				// Remove the broken container so CreateNode below can succeed
				_ = h.Manager.DeleteNode(ctx, ctr.ID)
			} else {
				pid, err := h.Manager.GetNodePID(ctx, ctr.ID)
				if err != nil {
					h.Logger.Warn("failed to get PID after restart", "node", node.Name, "error", err)
				}
				h.Runtime.Set(node.ID, ctr.ID, pid)
				h.storeServicePorts(ctx, &node, ctr.ID)
				node.ContainerID = ctr.ID
				node.PID = pid
				nodeMap[node.ID] = node
				revived++
				continue
			}
		}

		// Missing → create container
		cid, err := h.Manager.CreateNode(ctx, node)
		if err != nil {
			h.Logger.Warn("failed to revive node", "node", node.Name, "error", err)
			failed++
			continue
		}

		pid, err := h.Manager.GetNodePID(ctx, cid)
		if err != nil {
			h.Logger.Warn("failed to get PID for revived node", "node", node.Name, "error", err)
		}
			h.Runtime.Set(node.ID, cid, pid)
			h.storeServicePorts(ctx, &node, cid)
			node.ContainerID = cid
			node.PID = pid
			nodeMap[node.ID] = node
			revived++
	}
	return
}

// restoreLinks recreates veth pairs between nodes using the pre-built nodeMap
func (h *Handler) restoreLinks(ctx context.Context, nodeMap map[string]models.Node) (restored, failed int) {
	links, err := h.Repo.ListLinks()
	if err != nil {
		h.Logger.Warn("failed to list links for restoration", "error", err)
		return
	}

	for _, l := range links {
		src, okS := nodeMap[l.SourceID]
		tgt, okT := nodeMap[l.TargetID]
		if !okS || !okT || src.PID == 0 || tgt.PID == 0 {
			failed++
			continue
		}

		if err := h.Network.CreateLink(l, src.PID, tgt.PID); err != nil {
			h.Logger.Debug("link restoration note", "link", l.ID, "error", err)
			failed++
			continue
		}
		restored++

		if models.NeedsBridge(src.Type) {
			_ = h.Manager.AttachInterfaceToBridge(ctx, src.ContainerID, l.SourceInt, src.Type)
		}
		if models.NeedsBridge(tgt.Type) {
			_ = h.Manager.AttachInterfaceToBridge(ctx, tgt.ContainerID, l.TargetInt, tgt.Type)
		}
	}
	return
}

// restoreIPsAndRoutes reapplies saved IP and route configurations using the pre-built nodeMap
func (h *Handler) restoreIPsAndRoutes(ctx context.Context, nodeMap map[string]models.Node) (restoredIPs, restoredRoutes int) {
	labs, _ := h.Repo.ListLaboratories()

	for _, lab := range labs {
		// Restore IPs first
		configs, err := h.Repo.GetInterfaceConfigsByLab(lab.ID)
		if err != nil {
			continue
		}
		for _, cfg := range configs {
			node, ok := nodeMap[cfg.NodeID]
			if !ok || node.ContainerID == "" {
				continue
			}
			if err := h.Manager.ConfigureInterface(ctx, node.ContainerID, cfg.Interface, cfg.Address); err != nil {
				h.Logger.Warn("failed to restore IP", "node", node.Name, "iface", cfg.Interface, "error", err)
			} else {
				restoredIPs++
			}
		}

		// Then restore routes (routes depend on IPs being configured)
		routes, err := h.Repo.GetRouteConfigsByLab(lab.ID)
		if err != nil {
			continue
		}
		for _, cfg := range routes {
			node, ok := nodeMap[cfg.NodeID]
			if !ok || node.ContainerID == "" {
				continue
			}
			if err := h.Manager.ConfigureRoute(ctx, node.ContainerID, cfg.Dst, cfg.Gateway, cfg.Dev); err != nil {
				h.Logger.Warn("failed to restore route", "node", node.Name, "dst", cfg.Dst, "error", err)
			} else {
				restoredRoutes++
			}
		}
	}
	return
}

// SaveAllLabsState captures and persists IP and route configuration for all active laboratories.
// Acquires labOpMu to prevent racing with ActivateLaboratory or SaveLabState.
func (h *Handler) SaveAllLabsState(ctx context.Context) error {
	h.labOpMu.Lock()
	defer h.labOpMu.Unlock()
	return h.saveAllLabsStateLocked(ctx)
}

// saveAllLabsStateLocked is the internal version of SaveAllLabsState.
// The caller MUST hold h.labOpMu before calling this.
func (h *Handler) saveAllLabsStateLocked(ctx context.Context) error {
	labs, err := h.Repo.ListLaboratories()
	if err != nil {
		return fmt.Errorf("failed to list laboratories: %w", err)
	}

	totalIPs, totalRoutes := 0, 0
	for _, lab := range labs {
		nodes, err := h.Repo.ListNodesByLab(lab.ID)
		if err != nil {
			h.Logger.Warn("failed to list nodes for save state", "lab", lab.ID, "error", err)
			continue
		}
		h.hydrateNodes(nodes)

		configs, routeConfigs, hasRunning := h.captureLabState(ctx, lab.ID, nodes)
		if !hasRunning {
			continue
		}

		if err := h.Repo.SaveInterfaceConfigs(lab.ID, configs); err != nil {
			h.Logger.Warn("failed to save interface configs", "lab", lab.ID, "error", err)
			continue
		}
		if err := h.Repo.SaveRouteConfigs(lab.ID, routeConfigs); err != nil {
			h.Logger.Warn("failed to save route configs", "lab", lab.ID, "error", err)
			continue
		}
		totalIPs += len(configs)
		totalRoutes += len(routeConfigs)
	}

	h.Logger.Info("all labs state saved", "labs", len(labs), "ips_saved", totalIPs, "routes_saved", totalRoutes)
	return nil
}
