package handlers

import (
	"net"
	"net/http"
	"regexp"
	"strings"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
)

// TracerouteHop represents a single hop in a traceroute result
type TracerouteHop struct {
	Hop    int    `json:"hop"`
	IP     string `json:"ip"`
	RTT    string `json:"rtt"`
	NodeID string `json:"node_id,omitempty"`
}

// TracerouteResponse is the full traceroute result with resolved topology elements
type TracerouteResponse struct {
	Hops        []TracerouteHop `json:"hops"`
	NodeIDs     []string        `json:"node_ids"`
	LinkIDs     []string        `json:"link_ids"`
	Unreachable bool            `json:"unreachable,omitempty"`
}

// RunTraceroute executes traceroute from a node and resolves hops to topology elements
func (h *Handler) RunTraceroute(c *gin.Context) {
	id := c.Param("id")
	node, ok := h.getRunningNode(c, id)
	if !ok {
		return
	}

	var body struct {
		Destination string `json:"destination" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "destination is required"})
		return
	}

	// Validate destination is a valid IP
	if net.ParseIP(body.Destination) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
		return
	}

	// Build IP -> nodeID map from all nodes in the same lab
	ipToNodeID := make(map[string]string)
	labNodes, err := h.Repo.ListNodesByLab(node.LabID)
	if err != nil {
		h.Logger.Error("failed to list lab nodes for traceroute", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list lab nodes"})
		return
	}

	h.hydrateNodes(labNodes)
	for _, n := range labNodes {
		if n.ContainerID == "" {
			continue
		}
		ifaces, err := h.Manager.GetNodeInterfaces(c.Request.Context(), n.ContainerID)
		if err != nil {
			continue
		}
		for _, iface := range ifaces {
			if iface.Name == "lo" || iface.Name == "docker0" {
				continue
			}
			for _, addr := range iface.IPAddresses {
				// Strip any CIDR suffix and store bare IP
				ip := addr.Address
				if idx := strings.Index(ip, "/"); idx != -1 {
					ip = ip[:idx]
				}
				ipToNodeID[ip] = n.ID
			}
		}
	}

	// Execute traceroute
	output, err := h.Manager.RunTraceroute(c.Request.Context(), node.ContainerID, body.Destination)
	if err != nil {
		h.Logger.Error("traceroute failed", "node", node.Name, "dest", body.Destination, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "traceroute execution failed"})
		return
	}

	// Parse traceroute output
	hops := parseTracerouteOutput(output)

	// Resolve node IDs for each hop
	nodeIDSet := make(map[string]bool)
	nodeIDSet[id] = true // Include source node
	for i := range hops {
		if hops[i].IP != "*" {
			if nid, ok := ipToNodeID[hops[i].IP]; ok {
				hops[i].NodeID = nid
				nodeIDSet[nid] = true
			}
		}
	}

	// Build ordered node IDs list
	var nodeIDs []string
	nodeIDs = append(nodeIDs, id) // Source first
	for _, hop := range hops {
		if hop.NodeID != "" && hop.NodeID != id {
			nodeIDs = append(nodeIDs, hop.NodeID)
		}
	}

	// Resolve link IDs between consecutive nodes in the path.
	// Nodes may be connected through L2 devices (switches/hubs) that don't
	// appear in the traceroute, so we BFS through them.
	labLinks, _ := h.Repo.ListLinksByLab(node.LabID)
	nodeTypeMap := make(map[string]models.NodeType, len(labNodes))
	for _, n := range labNodes {
		nodeTypeMap[n.ID] = n.Type
	}

	// Build adjacency: nodeID -> [(neighborID, linkID)]
	adj := make(map[string][]graphNeighbor)
	for _, link := range labLinks {
		adj[link.SourceID] = append(adj[link.SourceID], graphNeighbor{link.TargetID, link.ID})
		adj[link.TargetID] = append(adj[link.TargetID], graphNeighbor{link.SourceID, link.ID})
	}

	var linkIDs []string
	for i := 0; i < len(nodeIDs)-1; i++ {
		linkIDs = append(linkIDs, findLinksBFS(adj, nodeTypeMap, nodeIDs[i], nodeIDs[i+1])...)
	}

	// Mark as unreachable when all hops timed out (no resolved IP)
	unreachable := len(hops) > 0
	for _, hop := range hops {
		if hop.IP != "*" {
			unreachable = false
			break
		}
	}

	c.JSON(http.StatusOK, TracerouteResponse{
		Hops:        hops,
		NodeIDs:     nodeIDs,
		LinkIDs:     linkIDs,
		Unreachable: unreachable,
	})
}

type graphNeighbor struct {
	nodeID string
	linkID string
}

// findLinksBFS finds the link IDs on the shortest path between two nodes,
// traversing through L2 devices (switches/hubs) that are invisible to traceroute.
func findLinksBFS(adj map[string][]graphNeighbor, nodeTypes map[string]models.NodeType, from, to string) []string {
	type step struct {
		nodeID  string
		linkIDs []string
	}
	visited := map[string]bool{from: true}
	queue := []step{{nodeID: from}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, nb := range adj[cur.nodeID] {
			if visited[nb.nodeID] {
				continue
			}
			path := append(append([]string{}, cur.linkIDs...), nb.linkID)
			if nb.nodeID == to {
				return path
			}
			// Only traverse through L2 devices (switch/hub)
			t := nodeTypes[nb.nodeID]
			if t == models.SWITCH || t == models.HUB {
				visited[nb.nodeID] = true
				queue = append(queue, step{nodeID: nb.nodeID, linkIDs: path})
			}
		}
	}
	return nil
}

// parseTracerouteOutput parses the raw output of `traceroute -n` into structured hops.
// Truncates after 3 consecutive unreachable (*) hops to avoid noise.
func parseTracerouteOutput(output string) []TracerouteHop {
	var hops []TracerouteHop

	// Match lines like: " 1  10.0.1.1  0.123 ms  0.456 ms  0.789 ms"
	// or: " 2  * * *"
	hopRegex := regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)

	// Match IP + RTT pairs like: "10.0.1.1  0.123 ms"
	ipRttRegex := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)\s+([\d.]+)\s*ms`)

	consecutiveTimeouts := 0

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		match := hopRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		hopNum := 0
		for _, c := range match[1] {
			hopNum = hopNum*10 + int(c-'0')
		}

		rest := match[2]
		hop := TracerouteHop{Hop: hopNum, IP: "*", RTT: "*"}

		// Try to extract IP and RTT
		ipMatch := ipRttRegex.FindStringSubmatch(rest)
		if ipMatch != nil {
			hop.IP = ipMatch[1]
			hop.RTT = ipMatch[2] + " ms"
			consecutiveTimeouts = 0
		} else if strings.Contains(rest, "*") {
			hop.IP = "*"
			hop.RTT = "*"
			consecutiveTimeouts++
		}

		hops = append(hops, hop)

		// Stop after 3 consecutive timeouts
		if consecutiveTimeouts >= 3 {
			break
		}
	}

	return hops
}
