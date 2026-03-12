package handlers

import (
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
)

// Domain represents a broadcast or collision domain with its member nodes and links.
type Domain struct {
	ID      int      `json:"id"`
	NodeIDs []string `json:"node_ids"`
	LinkIDs []string `json:"link_ids"`
}

type domainsResponse struct {
	BroadcastDomains []Domain `json:"broadcast_domains"`
	CollisionDomains []Domain `json:"collision_domains"`
}

// --- Union-Find ---

type unionFind struct {
	parent map[string]string
	rank   map[string]int
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[string]string),
		rank:   make(map[string]int),
	}
}

func (uf *unionFind) add(x string) {
	if _, ok := uf.parent[x]; !ok {
		uf.parent[x] = x
	}
}

func (uf *unionFind) find(x string) string {
	for uf.parent[x] != x {
		uf.parent[x] = uf.parent[uf.parent[x]] // path compression
		x = uf.parent[x]
	}
	return x
}

func (uf *unionFind) union(a, b string) {
	ra, rb := uf.find(a), uf.find(b)
	if ra == rb {
		return
	}
	if uf.rank[ra] < uf.rank[rb] {
		ra, rb = rb, ra
	}
	uf.parent[rb] = ra
	if uf.rank[ra] == uf.rank[rb] {
		uf.rank[ra]++
	}
}

// --- Algorithm ---

// computeDomains is the shared engine for both broadcast and collision domain
// computation. The usesBaseKey predicate controls which node types use a single
// UF key (base node ID) vs per-link virtual keys (nodeID:linkID).
//
// Nodes with base keys are "transparent" — all their ports merge into one domain.
// Nodes with per-link keys "separate" — each interface stays in its own domain.
func computeDomains(nodes []models.Node, links []models.Link, usesBaseKey func(models.NodeType) bool) []Domain {
	if len(nodes) == 0 {
		return nil
	}

	nodeType := make(map[string]models.NodeType, len(nodes))
	for _, n := range nodes {
		nodeType[n.ID] = n.Type
	}

	linkEndpoint := func(nodeID, linkID string) string {
		if usesBaseKey(nodeType[nodeID]) {
			return nodeID
		}
		return nodeID + ":" + linkID
	}

	uf := newUnionFind()

	// Track which UF keys belong to each real node.
	nodeKeys := make(map[string][]string, len(nodes))

	// Only add base keys for nodes that use them.
	for _, n := range nodes {
		if usesBaseKey(n.Type) {
			uf.add(n.ID)
			nodeKeys[n.ID] = append(nodeKeys[n.ID], n.ID)
		}
	}

	// Process links: create UF keys and union them.
	for _, l := range links {
		srcKey := linkEndpoint(l.SourceID, l.ID)
		tgtKey := linkEndpoint(l.TargetID, l.ID)
		uf.add(srcKey)
		uf.add(tgtKey)
		uf.union(srcKey, tgtKey)

		// Track per-link keys for nodes that don't use base keys.
		if !usesBaseKey(nodeType[l.SourceID]) {
			nodeKeys[l.SourceID] = append(nodeKeys[l.SourceID], srcKey)
		}
		if !usesBaseKey(nodeType[l.TargetID]) {
			nodeKeys[l.TargetID] = append(nodeKeys[l.TargetID], tgtKey)
		}
	}

	// Collect domains by root.
	rootToNodes := make(map[string]map[string]bool)
	rootToLinks := make(map[string]map[string]bool)

	for _, n := range nodes {
		keys := nodeKeys[n.ID]
		if len(keys) == 0 {
			// Per-link node with no links — isolated node, own domain.
			if rootToNodes[n.ID] == nil {
				rootToNodes[n.ID] = make(map[string]bool)
			}
			rootToNodes[n.ID][n.ID] = true
			continue
		}
		for _, key := range keys {
			root := uf.find(key)
			if rootToNodes[root] == nil {
				rootToNodes[root] = make(map[string]bool)
			}
			rootToNodes[root][n.ID] = true
		}
	}

	for _, l := range links {
		srcKey := linkEndpoint(l.SourceID, l.ID)
		root := uf.find(srcKey)
		if rootToLinks[root] == nil {
			rootToLinks[root] = make(map[string]bool)
		}
		rootToLinks[root][l.ID] = true
	}

	return buildDomains(rootToNodes, rootToLinks)
}

// computeBroadcastDomains calculates broadcast domains from topology.
// Switches, hubs, and hosts use base keys (transparent / end-device).
// Routers and clouds use per-link keys (each interface = own domain).
func computeBroadcastDomains(nodes []models.Node, links []models.Link) []Domain {
	return computeDomains(nodes, links, func(t models.NodeType) bool {
		return t != models.ROUTER && t != models.CLOUD
	})
}

// computeCollisionDomains calculates collision domains from topology.
// Only hubs use base keys (shared medium — all ports in one collision domain).
// Everything else uses per-link keys (each link segment = own collision domain).
func computeCollisionDomains(nodes []models.Node, links []models.Link) []Domain {
	return computeDomains(nodes, links, func(t models.NodeType) bool {
		return t == models.HUB
	})
}

// buildDomains converts root->members maps into Domain slices.
func buildDomains(rootToNodes map[string]map[string]bool, rootToLinks map[string]map[string]bool) []Domain {
	var domains []Domain
	id := 0
	for root, nodeSet := range rootToNodes {
		nodeIDs := make([]string, 0, len(nodeSet))
		for nid := range nodeSet {
			nodeIDs = append(nodeIDs, nid)
		}
		linkIDs := make([]string, 0)
		if ls, ok := rootToLinks[root]; ok {
			for lid := range ls {
				linkIDs = append(linkIDs, lid)
			}
		}
		domains = append(domains, Domain{
			ID:      id,
			NodeIDs: nodeIDs,
			LinkIDs: linkIDs,
		})
		id++
	}
	return domains
}

// --- HTTP Handler ---

// GetDomains computes broadcast and collision domains for a laboratory.
func (h *Handler) GetDomains(c *gin.Context) {
	labID := c.Param("id")

	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for domains", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	links, err := h.Repo.ListLinksByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list links for domains", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	broadcast := computeBroadcastDomains(nodes, links)
	collision := computeCollisionDomains(nodes, links)

	if broadcast == nil {
		broadcast = []Domain{}
	}
	if collision == nil {
		collision = []Domain{}
	}

	c.JSON(http.StatusOK, domainsResponse{
		BroadcastDomains: broadcast,
		CollisionDomains: collision,
	})
}
