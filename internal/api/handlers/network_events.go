package handlers

import (
	"sync"

	"github.com/gorilla/websocket"
)

// NetworkEvent represents a change in network configuration
type NetworkEvent struct {
	Type      string      `json:"type"` // e.g., "interface:changed", "node:created", "link:created"
	NodeID    string      `json:"nodeId"`
	LabID     string      `json:"labId,omitempty"`
	Timestamp int64       `json:"timestamp"` // Unix timestamp
	Data      interface{} `json:"data,omitempty"`
}

// NetworkEventHub manages WebSocket connections for network events
type NetworkEventHub struct {
	mu         sync.RWMutex
	clients    map[*websocket.Conn]bool
	broadcast  chan NetworkEvent
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	done       chan struct{}
}

// NewNetworkEventHub creates a new event hub
func NewNetworkEventHub() *NetworkEventHub {
	return &NetworkEventHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan NetworkEvent, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's event loop. Blocks until Stop is called.
func (h *NetworkEventHub) Run() {
	for {
		select {
		case <-h.done:
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			// Copy client list to avoid holding the lock during writes
			h.mu.RLock()
			clients := make([]*websocket.Conn, 0, len(h.clients))
			for c := range h.clients {
				clients = append(clients, c)
			}
			h.mu.RUnlock()

			for _, client := range clients {
				if err := client.WriteJSON(event); err != nil {
					h.unregister <- client
				}
			}
		}
	}
}

// Stop signals the hub to shut down and close all connections
func (h *NetworkEventHub) Stop() {
	close(h.done)
}

// Broadcast sends an event to all connected clients
func (h *NetworkEventHub) Broadcast(event NetworkEvent) {
	select {
	case h.broadcast <- event:
	default:
		// Channel full, drop event (prevents blocking)
	}
}

// Register adds a new WebSocket client
func (h *NetworkEventHub) Register(conn *websocket.Conn) {
	h.register <- conn
}

// Unregister removes a WebSocket client
func (h *NetworkEventHub) Unregister(conn *websocket.Conn) {
	h.unregister <- conn
}

// ClientCount returns the number of connected clients
func (h *NetworkEventHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
