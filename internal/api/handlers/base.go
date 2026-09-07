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
	if node.Type != models.MONITOR && node.Type != models.HAPROXY && node.Type != models.ZABBIX {
		return
	}

	if node.Type == models.ZABBIX {
		ports, err := h.Manager.GetServicePorts(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get zabbix ports", "node", node.Name, "error", err)
			return
		}
		zabbixPort, ok := ports["zabbix"]
		if !ok {
			return
		}
		nodeID := node.ID
		nodeName := node.Name
		labID := node.LabID
		go func() {
			h.Logger.Info("waiting for zabbix web UI to be ready", "node", nodeName)

			// Phase 1: poll systemctl until mariadb → zabbix-server → apache2 are all active.
			// Each service takes time; broadcast the current bottleneck so the panel shows
			// a meaningful stage instead of a generic "starting..." message.
			type svcStage struct {
				name  string
				label string
			}
			stages := []svcStage{
				{"mariadb", "Starting MariaDB..."},
				{"zabbix-init", "Initializing database..."},
				{"zabbix-server", "Starting Zabbix Server..."},
				{"apache2", "Starting Apache..."},
			}
			checkActive := func(service string) bool {
				execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := h.Manager.ExecCommand(execCtx, containerID, []string{"systemctl", "is-active", service})
				return err == nil
			}
			ticker := time.NewTicker(3 * time.Second)
			deadline := time.Now().Add(4 * time.Minute)
		serviceLoop:
			for time.Now().Before(deadline) {
				<-ticker.C
				allActive := true
				for _, s := range stages {
					if !checkActive(s.name) {
						h.broadcastServiceStatus(nodeID, labID, s.label)
						allActive = false
						break
					}
				}
				if allActive {
					break serviceLoop
				}
			}
			ticker.Stop()

			// Phase 2: wait for the web UI to respond on port 80.
			h.broadcastServiceStatus(nodeID, labID, "Waiting for web UI...")
			if err := h.Manager.WaitForHTTP(context.Background(), containerID, 80, 60*time.Second); err != nil {
				h.Logger.Warn("zabbix did not become ready", "node", nodeName, "error", err)
				h.broadcastServiceStatus(nodeID, labID, "")
				return
			}

			h.Logger.Info("zabbix ready", "node", nodeName, "port", zabbixPort)
			h.broadcastServiceStatus(nodeID, labID, "")
			current, _ := h.Runtime.Get(nodeID)
			merged := map[string]int{}
			for k, v := range current.ServicePorts {
				merged[k] = v
			}
			merged["zabbix"] = zabbixPort
			h.Runtime.SetServicePorts(nodeID, merged)
			h.EventHub.Broadcast(NetworkEvent{
				Type:      "node:updated",
				NodeID:    nodeID,
				LabID:     labID,
				Timestamp: time.Now().Unix(),
			})
		}()
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
		h.broadcastServiceStatus(nodeID, labID, "Waiting for Grafana...")
		if err := h.Manager.WaitForHTTP(context.Background(), containerID, 3000, 3*time.Minute); err != nil {
			h.Logger.Warn("grafana did not become ready", "node", nodeName, "error", err)
			h.broadcastServiceStatus(nodeID, labID, "")
			return
		}
		h.Logger.Info("grafana ready", "node", nodeName, "port", grafanaPort)
		h.broadcastServiceStatus(nodeID, labID, "")
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

// broadcastServiceStatus sends a node:service_status event so the frontend can show
// real-time startup progress (e.g. "Starting MariaDB...", "Waiting for Grafana...").
// An empty message signals that the node is ready and the status should be cleared.
func (h *Handler) broadcastServiceStatus(nodeID, labID, message string) {
	h.EventHub.Broadcast(NetworkEvent{
		Type:      "node:service_status",
		NodeID:    nodeID,
		LabID:     labID,
		Data:      map[string]string{"message": message},
		Timestamp: time.Now().Unix(),
	})
}
