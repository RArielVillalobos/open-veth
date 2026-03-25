package handlers

import "sync"

// RuntimeState holds ephemeral Docker state for a node
type RuntimeState struct {
	ContainerID  string
	PID          int
	ServicePorts map[string]int // only populated for MONITOR nodes
}

// RuntimeStore is a thread-safe in-memory store for node runtime state.
// This data is NOT persisted — it is rebuilt from Docker on startup via reconciliation.
type RuntimeStore struct {
	mu    sync.RWMutex
	nodes map[string]RuntimeState
}

// NewRuntimeStore creates an empty RuntimeStore
func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{
		nodes: make(map[string]RuntimeState),
	}
}

// Set stores runtime state for a node
func (r *RuntimeStore) Set(nodeID, containerID string, pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[nodeID] = RuntimeState{ContainerID: containerID, PID: pid}
}

// SetServicePorts stores the mapped host ports for a MONITOR node.
// If the node is not yet in the store it is upserted with only the ports set.
func (r *RuntimeStore) SetServicePorts(nodeID string, ports map[string]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.nodes[nodeID] // zero value if missing
	state.ServicePorts = ports
	r.nodes[nodeID] = state
}

// Get retrieves runtime state for a node
func (r *RuntimeStore) Get(nodeID string) (RuntimeState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.nodes[nodeID]
	return s, ok
}

// Delete removes runtime state for a node
func (r *RuntimeStore) Delete(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, nodeID)
}

// Clear removes all runtime state
func (r *RuntimeStore) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes = make(map[string]RuntimeState)
}
