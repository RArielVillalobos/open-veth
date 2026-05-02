package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"open-veth/internal/config"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"

	"github.com/gin-gonic/gin"
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

	// reconciling is set while ReconcileState is rebuilding containers.
	// The Docker watcher ignores events during this window.
	reconciling atomic.Bool
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

// IsReconciling returns true if ReconcileState is currently rebuilding containers.
func (h *Handler) IsReconciling() bool {
	return h.reconciling.Load()
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

// getRunningNode fetches and hydrates a node by ID. Writes a 404 if not found
// or a 503 if the node has no running container. Returns (node, false) on error.
func (h *Handler) getRunningNode(c *gin.Context, id string) (models.Node, bool) {
	node, found := h.Repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return models.Node{}, false
	}
	h.hydrateNode(&node)
	if node.ContainerID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
		return models.Node{}, false
	}
	return node, true
}

// storeServicePorts retrieves the dynamically assigned host ports for nodes that
// expose services (MONITOR and HAPROXY). For MONITOR it stores Prometheus
// immediately and waits async for Grafana. For HAPROXY it stores the stats port.
func (h *Handler) storeServicePorts(ctx context.Context, node *models.Node, containerID string) {
	if node.Type != models.MONITOR && node.Type != models.HAPROXY {
		return
	}

	if node.Type == models.HAPROXY {
		ports, err := h.Manager.GetServicePorts(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get haproxy ports", "node", node.Name, "error", err)
			return
		}
		if statsPort, ok := ports["stats"]; ok {
			servicePorts := map[string]int{"stats": statsPort}
			h.Runtime.SetServicePorts(node.ID, servicePorts)
			node.ServicePorts = servicePorts
			h.Logger.Info("haproxy stats ready", "node", node.Name, "port", statsPort)
		}
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
