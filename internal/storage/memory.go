package storage

import (
	"fmt"
	"open-veth/internal/models"
	"sync"
)

type MemoryRepository struct {
	nodes          map[string]models.Node
	links          map[string]models.Link
	labs           map[string]models.Laboratory
	interfaceConfs map[string][]models.InterfaceConfig // keyed by labID
	routeConfs     map[string][]models.RouteConfig     // keyed by labID
	mu             sync.RWMutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nodes:          make(map[string]models.Node),
		links:          make(map[string]models.Link),
		labs:           make(map[string]models.Laboratory),
		interfaceConfs: make(map[string][]models.InterfaceConfig),
		routeConfs:     make(map[string][]models.RouteConfig),
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
	node, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node not found")
	}

	// Cascade: delete links referencing this node
	for linkID, link := range m.links {
		if link.SourceID == id || link.TargetID == id {
			delete(m.links, linkID)
		}
	}

	// Cascade: remove interface configs for this node
	if confs, exists := m.interfaceConfs[node.LabID]; exists {
		filtered := make([]models.InterfaceConfig, 0, len(confs))
		for _, cfg := range confs {
			if cfg.NodeID != id {
				filtered = append(filtered, cfg)
			}
		}
		m.interfaceConfs[node.LabID] = filtered
	}

	// Cascade: remove route configs for this node
	if confs, exists := m.routeConfs[node.LabID]; exists {
		filtered := make([]models.RouteConfig, 0, len(confs))
		for _, cfg := range confs {
			if cfg.NodeID != id {
				filtered = append(filtered, cfg)
			}
		}
		m.routeConfs[node.LabID] = filtered
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

func (m *MemoryRepository) ListLinksByNode(nodeID string) ([]models.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []models.Link
	for _, l := range m.links {
		if l.SourceID == nodeID || l.TargetID == nodeID {
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

	// Cascade: delete configs, links, nodes
	delete(m.interfaceConfs, id)
	delete(m.routeConfs, id)

	for linkID, link := range m.links {
		if link.LabID == id {
			delete(m.links, linkID)
		}
	}
	for nodeID, node := range m.nodes {
		if node.LabID == id {
			delete(m.nodes, nodeID)
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
	m.interfaceConfs = make(map[string][]models.InterfaceConfig)
	m.routeConfs = make(map[string][]models.RouteConfig)
	return nil
}

func (m *MemoryRepository) SaveInterfaceConfigs(labID string, configs []models.InterfaceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interfaceConfs[labID] = configs
	return nil
}

func (m *MemoryRepository) GetInterfaceConfigsByLab(labID string) ([]models.InterfaceConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interfaceConfs[labID], nil
}

func (m *MemoryRepository) SaveRouteConfigs(labID string, configs []models.RouteConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routeConfs[labID] = configs
	return nil
}

func (m *MemoryRepository) GetRouteConfigsByLab(labID string) ([]models.RouteConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.routeConfs[labID], nil
}
