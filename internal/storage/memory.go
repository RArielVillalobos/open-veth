package storage

import (
	"fmt"
	"open-veth/internal/models"
	"sync"
)

type MemoryRepository struct {
	nodes map[string]models.Node
	links map[string]models.Link
	mu    sync.RWMutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nodes: make(map[string]models.Node),
		links: make(map[string]models.Link),
	}
}

// --- Nodos ---

func (m *MemoryRepository) SaveNode(node models.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
	return nil
}

func (m *MemoryRepository) GetNode(id string) (models.Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[id]
	return n, ok
}

func (m *MemoryRepository) DeleteNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nodes[id]; !ok {
		return fmt.Errorf("nodo no encontrado")
	}
	delete(m.nodes, id)
	return nil
}

func (m *MemoryRepository) ListNodes() ([]models.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]models.Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		list = append(list, n)
	}
	return list, nil
}

// --- Links ---

func (m *MemoryRepository) SaveLink(link models.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[link.ID] = link
	return nil
}

func (m *MemoryRepository) GetLink(id string) (models.Link, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.links[id]
	return l, ok
}

func (m *MemoryRepository) DeleteLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.links[id]; !ok {
		return fmt.Errorf("link no encontrado")
	}
	delete(m.links, id)
	return nil
}

func (m *MemoryRepository) ListLinks() ([]models.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]models.Link, 0, len(m.links))
	for _, l := range m.links {
		list = append(list, l)
	}
	return list, nil
}

func (m *MemoryRepository) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = make(map[string]models.Node)
	m.links = make(map[string]models.Link)
	return nil
}
