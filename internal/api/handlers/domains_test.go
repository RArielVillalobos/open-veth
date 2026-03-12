package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"open-veth/internal/models"
)

// --- Pure algorithm tests ---

func sortDomain(d *Domain) {
	sort.Strings(d.NodeIDs)
	sort.Strings(d.LinkIDs)
}

func sortDomains(ds []Domain) {
	for i := range ds {
		sortDomain(&ds[i])
	}
	sort.Slice(ds, func(i, j int) bool {
		if len(ds[i].NodeIDs) == 0 {
			return true
		}
		if len(ds[j].NodeIDs) == 0 {
			return false
		}
		return ds[i].NodeIDs[0] < ds[j].NodeIDs[0]
	})
}

func TestDomainsEmptyLab(t *testing.T) {
	bd := computeBroadcastDomains(nil, nil)
	cd := computeCollisionDomains(nil, nil)
	if bd != nil {
		t.Errorf("expected nil broadcast domains, got %v", bd)
	}
	if cd != nil {
		t.Errorf("expected nil collision domains, got %v", cd)
	}
}

func TestDomainsSingleIsolatedNode(t *testing.T) {
	nodes := []models.Node{{ID: "pc1", Type: models.HOST}}

	bd := computeBroadcastDomains(nodes, nil)
	cd := computeCollisionDomains(nodes, nil)

	if len(bd) != 1 {
		t.Fatalf("expected 1 broadcast domain, got %d", len(bd))
	}
	if len(cd) != 1 {
		t.Fatalf("expected 1 collision domain, got %d", len(cd))
	}
	if bd[0].NodeIDs[0] != "pc1" {
		t.Errorf("expected pc1 in broadcast domain, got %v", bd[0].NodeIDs)
	}
	if len(bd[0].LinkIDs) != 0 {
		t.Errorf("expected 0 links in broadcast domain, got %v", bd[0].LinkIDs)
	}
}

func TestDomainsTwoHostsLinked(t *testing.T) {
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "pc2"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// 1 broadcast domain containing both hosts and the link
	if len(bd) != 1 {
		t.Fatalf("expected 1 broadcast domain, got %d: %+v", len(bd), bd)
	}
	sortDomain(&bd[0])
	if len(bd[0].NodeIDs) != 2 {
		t.Errorf("expected 2 nodes in broadcast domain, got %v", bd[0].NodeIDs)
	}
	if len(bd[0].LinkIDs) != 1 || bd[0].LinkIDs[0] != "l1" {
		t.Errorf("expected link l1 in broadcast domain, got %v", bd[0].LinkIDs)
	}

	// 1 collision domain (single shared link)
	if len(cd) != 1 {
		t.Fatalf("expected 1 collision domain, got %d: %+v", len(cd), cd)
	}
	sortDomain(&cd[0])
	if len(cd[0].NodeIDs) != 2 {
		t.Errorf("expected 2 nodes in collision domain, got %v", cd[0].NodeIDs)
	}
}

func TestDomainsTwoHostsViaSwitch(t *testing.T) {
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "sw1", Type: models.SWITCH},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "sw1"},
		{ID: "l2", SourceID: "pc2", TargetID: "sw1"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Switch is transparent for broadcast: 1 broadcast domain with all 3 nodes
	if len(bd) != 1 {
		t.Fatalf("expected 1 broadcast domain, got %d: %+v", len(bd), bd)
	}
	sortDomain(&bd[0])
	if len(bd[0].NodeIDs) != 3 {
		t.Errorf("expected 3 nodes in broadcast domain, got %v", bd[0].NodeIDs)
	}
	if len(bd[0].LinkIDs) != 2 {
		t.Errorf("expected 2 links in broadcast domain, got %v", bd[0].LinkIDs)
	}

	// Switch separates collision domains: 2 collision domains
	// Each collision domain has 1 link and 2 nodes (host + switch)
	if len(cd) != 2 {
		t.Fatalf("expected 2 collision domains, got %d: %+v", len(cd), cd)
	}
	sortDomains(cd)
	for _, d := range cd {
		if len(d.LinkIDs) != 1 {
			t.Errorf("expected 1 link per collision domain, got %v", d.LinkIDs)
		}
		if len(d.NodeIDs) != 2 {
			t.Errorf("expected 2 nodes per collision domain, got %v", d.NodeIDs)
		}
	}
}

func TestDomainsTwoHostsViaRouter(t *testing.T) {
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "r1", Type: models.ROUTER},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "r1"},
		{ID: "l2", SourceID: "pc2", TargetID: "r1"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Router separates broadcast domains: 2 broadcast domains (one per interface)
	if len(bd) != 2 {
		t.Fatalf("expected 2 broadcast domains, got %d: %+v", len(bd), bd)
	}
	sortDomains(bd)
	for _, d := range bd {
		if len(d.LinkIDs) != 1 {
			t.Errorf("expected 1 link per broadcast domain, got %v", d.LinkIDs)
		}
		// Each broadcast domain has 2 nodes (host + router)
		if len(d.NodeIDs) != 2 {
			t.Errorf("expected 2 nodes per broadcast domain, got %v", d.NodeIDs)
		}
	}

	// Router also separates collision domains: 2 collision domains
	if len(cd) != 2 {
		t.Fatalf("expected 2 collision domains, got %d: %+v", len(cd), cd)
	}
}

func TestDomainsThreeHostsViaHub(t *testing.T) {
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "pc3", Type: models.HOST},
		{ID: "hub1", Type: models.HUB},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "hub1"},
		{ID: "l2", SourceID: "pc2", TargetID: "hub1"},
		{ID: "l3", SourceID: "pc3", TargetID: "hub1"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Hub is transparent for broadcast: 1 broadcast domain
	if len(bd) != 1 {
		t.Fatalf("expected 1 broadcast domain, got %d: %+v", len(bd), bd)
	}
	sortDomain(&bd[0])
	if len(bd[0].NodeIDs) != 4 {
		t.Errorf("expected 4 nodes in broadcast domain, got %v", bd[0].NodeIDs)
	}
	if len(bd[0].LinkIDs) != 3 {
		t.Errorf("expected 3 links in broadcast domain, got %v", bd[0].LinkIDs)
	}

	// Hub is transparent for collision: 1 collision domain (shared medium)
	if len(cd) != 1 {
		t.Fatalf("expected 1 collision domain, got %d: %+v", len(cd), cd)
	}
	sortDomain(&cd[0])
	if len(cd[0].NodeIDs) != 4 {
		t.Errorf("expected 4 nodes in collision domain, got %v", cd[0].NodeIDs)
	}
	if len(cd[0].LinkIDs) != 3 {
		t.Errorf("expected 3 links in collision domain, got %v", cd[0].LinkIDs)
	}
}

func TestDomainsMixedTopology(t *testing.T) {
	// Topology:  pc1 -- sw1 -- r1 -- sw2 -- pc2
	//                    |
	//                   pc3
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "pc3", Type: models.HOST},
		{ID: "sw1", Type: models.SWITCH},
		{ID: "sw2", Type: models.SWITCH},
		{ID: "r1", Type: models.ROUTER},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "sw1"},
		{ID: "l2", SourceID: "pc3", TargetID: "sw1"},
		{ID: "l3", SourceID: "sw1", TargetID: "r1"},
		{ID: "l4", SourceID: "r1", TargetID: "sw2"},
		{ID: "l5", SourceID: "sw2", TargetID: "pc2"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Router splits into 2 broadcast domains:
	//   BD1: pc1, pc3, sw1, r1 (links l1, l2, l3)
	//   BD2: pc2, sw2, r1 (links l4, l5)
	if len(bd) != 2 {
		t.Fatalf("expected 2 broadcast domains, got %d: %+v", len(bd), bd)
	}
	sortDomains(bd)

	// Find the domain that contains pc1
	var leftBD, rightBD Domain
	for _, d := range bd {
		sortDomain(&d)
		for _, nid := range d.NodeIDs {
			if nid == "pc1" {
				leftBD = d
			}
			if nid == "pc2" {
				rightBD = d
			}
		}
	}
	if len(leftBD.NodeIDs) != 4 {
		t.Errorf("left broadcast domain: expected 4 nodes (pc1,pc3,sw1,r1), got %v", leftBD.NodeIDs)
	}
	if len(leftBD.LinkIDs) != 3 {
		t.Errorf("left broadcast domain: expected 3 links, got %v", leftBD.LinkIDs)
	}
	if len(rightBD.NodeIDs) != 3 {
		t.Errorf("right broadcast domain: expected 3 nodes (pc2,sw2,r1), got %v", rightBD.NodeIDs)
	}
	if len(rightBD.LinkIDs) != 2 {
		t.Errorf("right broadcast domain: expected 2 links, got %v", rightBD.LinkIDs)
	}

	// Collision domains: each link between different non-hub devices = separate domain
	// Switches separate collision domains, so 5 collision domains (one per link)
	if len(cd) != 5 {
		t.Fatalf("expected 5 collision domains, got %d: %+v", len(cd), cd)
	}
	for _, d := range cd {
		if len(d.LinkIDs) != 1 {
			t.Errorf("expected 1 link per collision domain, got %v", d.LinkIDs)
		}
	}
}

func TestDomainsCloudNode(t *testing.T) {
	// Cloud behaves like router: separates both broadcast and collision domains
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "cloud1", Type: models.CLOUD},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "cloud1"},
		{ID: "l2", SourceID: "pc2", TargetID: "cloud1"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Cloud separates broadcast domains: 2
	if len(bd) != 2 {
		t.Fatalf("expected 2 broadcast domains, got %d: %+v", len(bd), bd)
	}

	// Cloud separates collision domains: 2
	if len(cd) != 2 {
		t.Fatalf("expected 2 collision domains, got %d: %+v", len(cd), cd)
	}
}

func TestDomainsRingTopology(t *testing.T) {
	// Ring: pc1 -- sw1 -- pc2 -- sw2 -- pc1
	// All nodes connected in a loop via switches.
	// For broadcast: switches are transparent → all nodes in 1 BD
	// For collision: each link is a separate CD
	nodes := []models.Node{
		{ID: "pc1", Type: models.HOST},
		{ID: "pc2", Type: models.HOST},
		{ID: "sw1", Type: models.SWITCH},
		{ID: "sw2", Type: models.SWITCH},
	}
	links := []models.Link{
		{ID: "l1", SourceID: "pc1", TargetID: "sw1"},
		{ID: "l2", SourceID: "sw1", TargetID: "pc2"},
		{ID: "l3", SourceID: "pc2", TargetID: "sw2"},
		{ID: "l4", SourceID: "sw2", TargetID: "pc1"},
	}

	bd := computeBroadcastDomains(nodes, links)
	cd := computeCollisionDomains(nodes, links)

	// Switches are transparent: 1 broadcast domain containing all 4 nodes
	if len(bd) != 1 {
		t.Fatalf("expected 1 broadcast domain for ring, got %d: %+v", len(bd), bd)
	}
	sortDomain(&bd[0])
	if len(bd[0].NodeIDs) != 4 {
		t.Errorf("expected 4 nodes in broadcast domain, got %v", bd[0].NodeIDs)
	}
	if len(bd[0].LinkIDs) != 4 {
		t.Errorf("expected 4 links in broadcast domain, got %v", bd[0].LinkIDs)
	}

	// 4 collision domains (one per link segment)
	if len(cd) != 4 {
		t.Fatalf("expected 4 collision domains for ring, got %d: %+v", len(cd), cd)
	}
	for _, d := range cd {
		if len(d.LinkIDs) != 1 {
			t.Errorf("expected 1 link per collision domain, got %v", d.LinkIDs)
		}
	}
}

func TestDomainsIsolatedRouterNoLinks(t *testing.T) {
	// Router with no links: each per-link-key node with 0 links gets its own isolated domain
	nodes := []models.Node{
		{ID: "r1", Type: models.ROUTER},
		{ID: "pc1", Type: models.HOST},
	}

	bd := computeBroadcastDomains(nodes, nil)
	cd := computeCollisionDomains(nodes, nil)

	// Router uses per-link keys → isolated → own domain; HOST also isolated
	if len(bd) != 2 {
		t.Fatalf("expected 2 broadcast domains (1 per isolated node), got %d: %+v", len(bd), bd)
	}
	if len(cd) != 2 {
		t.Fatalf("expected 2 collision domains (1 per isolated node), got %d: %+v", len(cd), cd)
	}
	for _, d := range bd {
		if len(d.NodeIDs) != 1 {
			t.Errorf("expected 1 node per isolated domain, got %v", d.NodeIDs)
		}
		if len(d.LinkIDs) != 0 {
			t.Errorf("expected 0 links for isolated domain, got %v", d.LinkIDs)
		}
	}
}

// --- HTTP handler test ---

func TestGetDomainsHandler(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Setup lab with a simple topology: pc1 -- sw1 -- pc2
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Test Lab"})
	repo.SaveNode(models.Node{ID: "pc1", LabID: "lab-1", Type: models.HOST})
	repo.SaveNode(models.Node{ID: "pc2", LabID: "lab-1", Type: models.HOST})
	repo.SaveNode(models.Node{ID: "sw1", LabID: "lab-1", Type: models.SWITCH})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "pc1", TargetID: "sw1"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1", SourceID: "pc2", TargetID: "sw1"})

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-1/domains", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp domainsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.BroadcastDomains) != 1 {
		t.Errorf("expected 1 broadcast domain, got %d", len(resp.BroadcastDomains))
	}
	if len(resp.CollisionDomains) != 2 {
		t.Errorf("expected 2 collision domains, got %d", len(resp.CollisionDomains))
	}
}

func TestGetDomainsHandlerNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-999/domains", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing lab, got %d", w.Code)
	}
}

func TestGetDomainsHandlerEmptyLab(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Empty Lab"})

	w := doRequest(r, "GET", "/api/v1/laboratories/lab-1/domains", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp domainsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.BroadcastDomains) != 0 {
		t.Errorf("expected 0 broadcast domains, got %d", len(resp.BroadcastDomains))
	}
	if len(resp.CollisionDomains) != 0 {
		t.Errorf("expected 0 collision domains, got %d", len(resp.CollisionDomains))
	}
}
