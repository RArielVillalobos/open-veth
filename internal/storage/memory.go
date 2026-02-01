package storage

import (
	"fmt"
	"open-veth/internal/models"
	"sync"
)

type MemoryRepository struct {
	nodes map[string]models.Node
	links map[string]models.Link
	labs  map[string]models.Laboratory
	mu    sync.RWMutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nodes: make(map[string]models.Node),
		links: make(map[string]models.Link),
		labs:  make(map[string]models.Laboratory),
	}
}

// --- Nodes ---

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
		return fmt.Errorf("node not found")
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

func (m *MemoryRepository) ListNodesByLab(labID string) ([]models.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]models.Node, 0)
	for _, n := range m.nodes {
		if n.LabID == labID {
			list = append(list, n)
		}
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
		return fmt.Errorf("link not found")
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

func (m *MemoryRepository) ListLinksByLab(labID string) ([]models.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]models.Link, 0)
	for _, l := range m.links {
		if l.LabID == labID {
			list = append(list, l)
		}
	}
	return list, nil
}

// --- Laboratories ---

func (m *MemoryRepository) SaveLaboratory(lab models.Laboratory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labs[lab.ID] = lab
	return nil
}

func (m *MemoryRepository) GetLaboratory(id string) (models.Laboratory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.labs[id]
	return l, ok
}

func (m *MemoryRepository) ListLaboratories() ([]models.Laboratory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]models.Laboratory, 0, len(m.labs))
	for _, l := range m.labs {
		list = append(list, l)
	}
	return list, nil
}

func (m *MemoryRepository) DeleteLaboratory(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Delete associated nodes and links
	for nodeID, node := range m.nodes {
		if node.LabID == id {
			delete(m.nodes, nodeID)
		}
	}
	for linkID, link := range m.links {
		if link.LabID == id {
			delete(m.links, linkID)
		}
	}
	
	delete(m.labs, id)
	return nil
}

func (m *MemoryRepository) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = make(map[string]models.Node)
	m.links = make(map[string]models.Link)
	m.labs = make(map[string]models.Laboratory)
	return nil
}
