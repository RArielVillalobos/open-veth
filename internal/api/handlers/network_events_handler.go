package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
)

// HandleNetworkEvents manages WebSocket connections for real-time network events
func (h *Handler) HandleNetworkEvents(c *gin.Context) {
	h.Logger.Info("network events connection attempt", "remote", c.Request.RemoteAddr)

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.Logger.Error("failed to upgrade network events websocket", "error", err)
		return
	}

	// Register client with the hub
	h.EventHub.Register(ws)
	defer h.EventHub.Unregister(ws)

	h.Logger.Info("network events client connected", "clients", h.EventHub.ClientCount())

	// Keep connection alive until client disconnects
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}

	h.Logger.Info("network events client disconnected", "clients", h.EventHub.ClientCount())
}

// BroadcastInterfaceChanged emits an event when interfaces might have changed
func (h *Handler) BroadcastInterfaceChanged(nodeID, labID string) {
	if h.EventHub == nil {
		return
	}

	h.Logger.Debug("broadcasting interface:changed event", "node", nodeID, "lab", labID)

	h.EventHub.Broadcast(NetworkEvent{
		Type:      "interface:changed",
		NodeID:    nodeID,
		LabID:     labID,
		Timestamp: time.Now().Unix(),
	})
}

// BroadcastNodeCreated emits an event when a node is created
func (h *Handler) BroadcastNodeCreated(nodeID, labID, nodeType string) {
	if h.EventHub == nil {
		return
	}

	h.EventHub.Broadcast(NetworkEvent{
		Type:      "node:created",
		NodeID:    nodeID,
		LabID:     labID,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"type": nodeType,
		},
	})
}

// BroadcastLinkCreated emits an event when a link is created
func (h *Handler) BroadcastLinkCreated(linkID, labID, sourceID, targetID string) {
	if h.EventHub == nil {
		return
	}

	h.EventHub.Broadcast(NetworkEvent{
		Type:      "link:created",
		NodeID:    "", // Links don't belong to a single node
		LabID:     labID,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"linkId":   linkID,
			"sourceId": sourceID,
			"targetId": targetID,
		},
	})
}

// BroadcastNodeStopped emits an event when a node is stopped
func (h *Handler) BroadcastNodeStopped(nodeID, labID string) {
	if h.EventHub == nil {
		return
	}
	h.EventHub.Broadcast(NetworkEvent{
		Type:      "node:stopped",
		NodeID:    nodeID,
		LabID:     labID,
		Timestamp: time.Now().Unix(),
	})
}

// BroadcastNodeStarted emits an event when a node is started
func (h *Handler) BroadcastNodeStarted(nodeID, labID string) {
	if h.EventHub == nil {
		return
	}
	h.EventHub.Broadcast(NetworkEvent{
		Type:      "node:started",
		NodeID:    nodeID,
		LabID:     labID,
		Timestamp: time.Now().Unix(),
	})
}

// BroadcastLinkToggled emits an event when a link is enabled or disabled
func (h *Handler) BroadcastLinkToggled(linkID, labID string, enabled bool) {
	if h.EventHub == nil {
		return
	}

	enabledStr := "false"
	if enabled {
		enabledStr = "true"
	}

	h.EventHub.Broadcast(NetworkEvent{
		Type:      "link:toggled",
		LabID:     labID,
		Timestamp: time.Now().Unix(),
		Data: map[string]string{
			"linkId":  linkID,
			"enabled": enabledStr,
		},
	})
}
