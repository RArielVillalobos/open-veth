package handlers

import (
	"net/http"
	"testing"

	"open-veth/internal/models"
)

// =============================================================================
// parseTracerouteOutput unit tests
// =============================================================================

func TestParseTracerouteOutput_HappyPath(t *testing.T) {
	output := `traceroute to 10.0.3.2 (10.0.3.2), 15 hops max, 46 byte packets
 1  10.0.1.1  0.016 ms  0.012 ms  0.011 ms
 2  10.0.2.2  0.009 ms  0.009 ms  0.011 ms
 3  10.0.3.2  0.010 ms  0.022 ms  0.008 ms
`
	hops := parseTracerouteOutput(output)

	if len(hops) != 3 {
		t.Fatalf("expected 3 hops, got %d", len(hops))
	}

	tests := []struct {
		hop int
		ip  string
	}{
		{1, "10.0.1.1"},
		{2, "10.0.2.2"},
		{3, "10.0.3.2"},
	}

	for i, tt := range tests {
		if hops[i].Hop != tt.hop {
			t.Errorf("hop[%d].Hop = %d, want %d", i, hops[i].Hop, tt.hop)
		}
		if hops[i].IP != tt.ip {
			t.Errorf("hop[%d].IP = %q, want %q", i, hops[i].IP, tt.ip)
		}
		if hops[i].RTT == "*" || hops[i].RTT == "" {
			t.Errorf("hop[%d].RTT should have a value, got %q", i, hops[i].RTT)
		}
	}
}

func TestParseTracerouteOutput_TimeoutsOnly(t *testing.T) {
	output := `traceroute to 10.0.3.2 (10.0.3.2), 15 hops max, 46 byte packets
 1  *  *  *
 2  *  *  *
 3  *  *  *
 4  *  *  *
 5  *  *  *
`
	hops := parseTracerouteOutput(output)

	// Should truncate after 3 consecutive timeouts
	if len(hops) != 3 {
		t.Fatalf("expected 3 hops (truncated), got %d", len(hops))
	}

	for i, hop := range hops {
		if hop.IP != "*" {
			t.Errorf("hop[%d].IP = %q, want '*'", i, hop.IP)
		}
		if hop.RTT != "*" {
			t.Errorf("hop[%d].RTT = %q, want '*'", i, hop.RTT)
		}
	}
}

func TestParseTracerouteOutput_TruncatesAfterThreeConsecutiveTimeouts(t *testing.T) {
	output := `traceroute to 10.0.3.2 (10.0.3.2), 15 hops max, 46 byte packets
 1  10.0.1.1  0.016 ms  0.012 ms  0.011 ms
 2  *  *  *
 3  *  *  *
 4  *  *  *
 5  *  *  *
 6  *  *  *
`
	hops := parseTracerouteOutput(output)

	// 1 real hop + 3 timeouts = 4 total, then truncate
	if len(hops) != 4 {
		t.Fatalf("expected 4 hops (1 real + 3 timeouts), got %d", len(hops))
	}

	if hops[0].IP != "10.0.1.1" {
		t.Errorf("hop[0].IP = %q, want '10.0.1.1'", hops[0].IP)
	}
	for i := 1; i <= 3; i++ {
		if hops[i].IP != "*" {
			t.Errorf("hop[%d].IP = %q, want '*'", i, hops[i].IP)
		}
	}
}

func TestParseTracerouteOutput_TimeoutThenRecovery(t *testing.T) {
	// A real hop after timeouts should reset the counter
	output := `traceroute to 10.0.3.2 (10.0.3.2), 15 hops max, 46 byte packets
 1  10.0.1.1  0.016 ms  0.012 ms  0.011 ms
 2  *  *  *
 3  *  *  *
 4  10.0.2.2  0.009 ms  0.009 ms  0.011 ms
 5  10.0.3.2  0.010 ms  0.022 ms  0.008 ms
`
	hops := parseTracerouteOutput(output)

	if len(hops) != 5 {
		t.Fatalf("expected 5 hops (recovery after timeouts), got %d", len(hops))
	}

	if hops[3].IP != "10.0.2.2" {
		t.Errorf("hop[3].IP = %q, want '10.0.2.2'", hops[3].IP)
	}
	if hops[4].IP != "10.0.3.2" {
		t.Errorf("hop[4].IP = %q, want '10.0.3.2'", hops[4].IP)
	}
}

func TestParseTracerouteOutput_EmptyOutput(t *testing.T) {
	hops := parseTracerouteOutput("")
	if len(hops) != 0 {
		t.Errorf("expected 0 hops for empty output, got %d", len(hops))
	}
}

func TestParseTracerouteOutput_DoubleDigitHops(t *testing.T) {
	output := `traceroute to 10.0.3.2 (10.0.3.2), 15 hops max, 46 byte packets
10  10.0.1.1  0.016 ms  0.012 ms  0.011 ms
11  10.0.2.2  0.009 ms  0.009 ms  0.011 ms
`
	hops := parseTracerouteOutput(output)

	if len(hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(hops))
	}
	if hops[0].Hop != 10 {
		t.Errorf("hop[0].Hop = %d, want 10", hops[0].Hop)
	}
	if hops[1].Hop != 11 {
		t.Errorf("hop[1].Hop = %d, want 11", hops[1].Hop)
	}
}

// =============================================================================
// findLinksBFS unit tests
// =============================================================================

// buildAdj is a helper to construct an adjacency map from a list of (src, dst, linkID) triples.
func buildAdj(edges [][3]string) map[string][]graphNeighbor {
	adj := make(map[string][]graphNeighbor)
	for _, e := range edges {
		src, dst, lid := e[0], e[1], e[2]
		adj[src] = append(adj[src], graphNeighbor{dst, lid})
		adj[dst] = append(adj[dst], graphNeighbor{src, lid})
	}
	return adj
}

func TestFindLinksBFS_DirectConnection(t *testing.T) {
	// pc1 -- pc2 (l1)
	adj := buildAdj([][3]string{{"pc1", "pc2", "l1"}})
	types := map[string]models.NodeType{"pc1": models.HOST, "pc2": models.HOST}

	links := findLinksBFS(adj, types, "pc1", "pc2")
	if len(links) != 1 || links[0] != "l1" {
		t.Errorf("expected [l1], got %v", links)
	}
}

func TestFindLinksBFS_ThroughSwitch(t *testing.T) {
	// pc1 -- sw1 -- r1: BFS traverses switch to reach router
	adj := buildAdj([][3]string{
		{"pc1", "sw1", "l1"},
		{"sw1", "r1", "l2"},
	})
	types := map[string]models.NodeType{
		"pc1": models.HOST,
		"sw1": models.SWITCH,
		"r1":  models.ROUTER,
	}

	links := findLinksBFS(adj, types, "pc1", "r1")
	if len(links) != 2 {
		t.Fatalf("expected 2 links through switch, got %v", links)
	}
	if links[0] != "l1" || links[1] != "l2" {
		t.Errorf("expected [l1, l2], got %v", links)
	}
}

func TestFindLinksBFS_ThroughHub(t *testing.T) {
	// pc1 -- hub1 -- r1: BFS traverses hub to reach router
	adj := buildAdj([][3]string{
		{"pc1", "hub1", "l1"},
		{"hub1", "r1", "l2"},
	})
	types := map[string]models.NodeType{
		"pc1":  models.HOST,
		"hub1": models.HUB,
		"r1":   models.ROUTER,
	}

	links := findLinksBFS(adj, types, "pc1", "r1")
	if len(links) != 2 {
		t.Fatalf("expected 2 links through hub, got %v", links)
	}
	if links[0] != "l1" || links[1] != "l2" {
		t.Errorf("expected [l1, l2], got %v", links)
	}
}

func TestFindLinksBFS_BlockedByNonL2Node(t *testing.T) {
	// pc1 -- r1 -- pc2: router blocks BFS, pc2 unreachable
	adj := buildAdj([][3]string{
		{"pc1", "r1", "l1"},
		{"r1", "pc2", "l2"},
	})
	types := map[string]models.NodeType{
		"pc1": models.HOST,
		"r1":  models.ROUTER,
		"pc2": models.HOST,
	}

	links := findLinksBFS(adj, types, "pc1", "pc2")
	if links != nil {
		t.Errorf("expected nil (blocked by router), got %v", links)
	}
}

func TestFindLinksBFS_NoPath(t *testing.T) {
	// Isolated nodes
	adj := map[string][]graphNeighbor{"pc1": {}, "pc2": {}}
	types := map[string]models.NodeType{"pc1": models.HOST, "pc2": models.HOST}

	links := findLinksBFS(adj, types, "pc1", "pc2")
	if links != nil {
		t.Errorf("expected nil for disconnected nodes, got %v", links)
	}
}

func TestFindLinksBFS_CyclicTopology_ShortestPath(t *testing.T) {
	// Ring: r1 -- sw1 -- r2 -- sw2 -- r1
	// Shortest path from r1 to r2 should go through sw1 (l1, l2)
	adj := buildAdj([][3]string{
		{"r1", "sw1", "l1"},
		{"sw1", "r2", "l2"},
		{"r2", "sw2", "l3"},
		{"sw2", "r1", "l4"},
	})
	types := map[string]models.NodeType{
		"r1":  models.ROUTER,
		"sw1": models.SWITCH,
		"r2":  models.ROUTER,
		"sw2": models.SWITCH,
	}

	links := findLinksBFS(adj, types, "r1", "r2")
	if len(links) != 2 {
		t.Fatalf("expected 2 links (shortest path), got %v", links)
	}
	if links[0] != "l1" || links[1] != "l2" {
		t.Errorf("expected [l1, l2] (via sw1), got %v", links)
	}
}

func TestFindLinksBFS_SameSourceAndDest(t *testing.T) {
	adj := buildAdj([][3]string{{"pc1", "pc2", "l1"}})
	types := map[string]models.NodeType{"pc1": models.HOST, "pc2": models.HOST}

	// Searching from a node to itself — should return nothing (visited from start)
	links := findLinksBFS(adj, types, "pc1", "pc1")
	if links != nil {
		t.Errorf("expected nil for same src/dst, got %v", links)
	}
}

// =============================================================================
// RunTraceroute handler tests
// =============================================================================

func TestTracerouteNodeNotFound(t *testing.T) {
	h, _ := newTestHandler()
	r := setupRouter(h)

	w := doRequest(r, "POST", "/api/v1/nodes/n999/traceroute", map[string]string{
		"destination": "10.0.1.1",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracerouteNodeNotRunning(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	// Node exists but no runtime state (not running)
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})

	w := doRequest(r, "POST", "/api/v1/nodes/n1/traceroute", map[string]string{
		"destination": "10.0.1.1",
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracerouteInvalidIP(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	// Invalid IP
	w := doRequest(r, "POST", "/api/v1/nodes/n1/traceroute", map[string]string{
		"destination": "not-an-ip",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid IP, got %d: %s", w.Code, w.Body.String())
	}

	// Empty destination
	w = doRequest(r, "POST", "/api/v1/nodes/n1/traceroute", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty destination, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracerouteMissingBody(t *testing.T) {
	h, repo := newTestHandler()
	r := setupRouter(h)

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1"})
	h.Runtime.Set("n1", "abc123", 1000)

	w := doRequest(r, "POST", "/api/v1/nodes/n1/traceroute", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing body, got %d: %s", w.Code, w.Body.String())
	}
}
