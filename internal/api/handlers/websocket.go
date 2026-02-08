package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow any origin in development
	},
}

// PacketSummary defines the lightweight structure sent to the UI
type PacketSummary struct {
	Timestamp   string `json:"timestamp"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	Length      int    `json:"length"`
	TTL         int    `json:"ttl"`
	Info        string `json:"info"`
}

// HandleTerminal manages WebSocket connection for terminal access
func (h *Handler) HandleTerminal(c *gin.Context) {
	nodeName := c.Query("node")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node name is required"})
		return
	}

	// SECURITY: Verify the node exists in our repository before allowing terminal access.
	node, found := h.Repo.GetNode(nodeName)
	if !found {
		// Also try lookup by name
		nodes, err := h.Repo.ListNodes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify node"})
			return
		}
		
		// Search for the node, prioritizing the one that is running if there are duplicates
		var candidates []models.Node
		for _, n := range nodes {
			if n.Name == nodeName || n.ID == nodeName {
				candidates = append(candidates, n)
			}
		}

		if len(candidates) > 0 {
			found = true
			node = candidates[0]
			// If we have multiple, try to find the one with a container_id
			for _, c := range candidates {
				h.hydrateNode(&c)
				if c.ContainerID != "" {
					node = c
					break
				}
			}
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	h.hydrateNode(&node)
	if node.ContainerID == "" {
		h.Logger.Warn("terminal request for stopped node", "name", node.Name, "id", node.ID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
		return
	}

	// 1. Upgrade HTTP to WebSocket
	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.Logger.Error("failed to upgrade to websocket", "error", err)
		return
	}
	defer ws.Close()

	// 2. Prepare command (standard bash for all nodes)
	shell := "bash"

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		Cmd:          []string{shell},
	}

	// 3. Create exec instance in container
	ctx := c.Request.Context()
	execID, err := h.Manager.GetDockerClient().ContainerExecCreate(ctx, node.ContainerID, execConfig)
	if err != nil {
		h.Logger.Error("failed to create exec", "node", node.Name, "error", err)
		return
	}

	// 4. Attach to exec (Hijack)
	resp, err := h.Manager.GetDockerClient().ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		h.Logger.Error("failed to attach to exec", "node", node.Name, "error", err)
		return
	}
	defer resp.Close()

	h.Logger.Info("terminal session started", "node", node.Name)

	// 5. Bidirectional data bridge

	// Output: Docker -> WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := resp.Reader.Read(buf)
			if n > 0 {
				if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Input: WebSocket -> Docker
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if _, err := resp.Conn.Write(msg); err != nil {
			break
		}
	}

	h.Logger.Info("terminal session ended", "node", node.Name)
}

// HandleSniff starts a live capture session over WebSockets
func (h *Handler) HandleSniff(c *gin.Context) {
	nodeID := c.Query("node_id")
	iface := c.Query("interface")

	if nodeID == "" || iface == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id and interface are required"})
		return
	}

	if err := models.ValidateInterfaceName(iface); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interface name"})
		return
	}

	node, found := h.Repo.GetNode(nodeID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	h.hydrateNode(&node)

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.Logger.Error("sniff websocket upgrade failed", "error", err)
		return
	}
	defer ws.Close()

	ctx := c.Request.Context()

	// 1. Prepare tcpdump command
	cmd := []string{"tcpdump", "-i", iface, "-w", "-", "-U", "-s", "0"}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	execID, err := h.Manager.GetDockerClient().ContainerExecCreate(ctx, node.ContainerID, execConfig)
	if err != nil {
		ws.WriteJSON(gin.H{"error": "failed to create sniffer: " + err.Error()})
		return
	}

	// 2. Start the capture stream
	resp, err := h.Manager.GetDockerClient().ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		ws.WriteJSON(gin.H{"error": "failed to start sniffer: " + err.Error()})
		return
	}
	defer resp.Close()

	// CLEANUP: Ensure tcpdump is killed when we exit
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), h.Config.Docker.CleanupTimeout)
		defer cancel()

		pattern := fmt.Sprintf("tcpdump -i %s", iface)
		if err := h.Manager.KillProcessByName(cleanupCtx, node.ContainerID, pattern); err != nil {
			h.Logger.Warn("failed to kill tcpdump process", "node", node.Name, "interface", iface, "error", err)
		}
		h.Logger.Info("sniffer cleanup completed", "node", node.Name, "interface", iface)
	}()

	// 3. Demultiplex Docker Stream
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		_, err := stdcopy.StdCopy(pw, io.Discard, resp.Reader)
		if err != nil {
			h.Logger.Warn("sniffer stream error", "error", err)
		}
	}()

	// 4. Use pcapgo to read from the Pipe
	pcapReader, err := pcapgo.NewReader(pr)
	if err != nil {
		h.Logger.Warn("pcap reader init error (might be empty stream)", "error", err)
		ws.WriteJSON(gin.H{"error": "pcap init error: " + err.Error()})
		return
	}

	h.Logger.Info("live capture started", "node", node.Name, "interface", iface)

	// 5. Packet processing loop
	for {
		data, ci, err := pcapReader.ReadPacketData()
		if err != nil {
			if err == io.EOF {
				break
			}
			h.Logger.Warn("packet read error", "error", err)
			break
		}

		packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
		summary := parsePacket(packet, ci.Timestamp)

		if err := ws.WriteJSON(summary); err != nil {
			break
		}
	}

	h.Logger.Info("live capture stopped", "node", node.Name, "interface", iface)
}

// parsePacket extracts human-readable info from raw packet data
func parsePacket(packet gopacket.Packet, ts time.Time) PacketSummary {
	summary := PacketSummary{
		Timestamp: ts.Format(time.RFC3339),
		Length:    len(packet.Data()),
		Protocol:  "L2",
		Info:      "Ethernet Frame",
	}

	// Layer 3: IP (v4 or v6)
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		summary.Source = ip.SrcIP.String()
		summary.Destination = ip.DstIP.String()
		summary.Protocol = ip.Protocol.String()
		summary.TTL = int(ip.TTL)
	} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv6)
		summary.Source = ip.SrcIP.String()
		summary.Destination = ip.DstIP.String()
		summary.Protocol = ip.NextHeader.String()
		summary.TTL = int(ip.HopLimit)
	}

	// Layer 4: TCP / UDP / ICMP
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		summary.Protocol = "TCP"
		flags := ""
		if tcp.SYN {
			flags += "S"
		}
		if tcp.ACK {
			flags += "A"
		}
		if tcp.FIN {
			flags += "F"
		}
		if tcp.RST {
			flags += "R"
		}
		if tcp.PSH {
			flags += "P"
		}
		summary.Info = fmt.Sprintf("%d → %d [%s] Seq=%d Ack=%d", tcp.SrcPort, tcp.DstPort, flags, tcp.Seq, tcp.Ack)
	} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		summary.Protocol = "UDP"
		summary.Info = fmt.Sprintf("%d → %d len=%d", udp.SrcPort, udp.DstPort, udp.Length)
	} else if icmpLayer := packet.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
		icmp, _ := icmpLayer.(*layers.ICMPv4)
		summary.Protocol = "ICMP"
		typeStr := "Unknown"
		if icmp.TypeCode.Type() == layers.ICMPv4TypeEchoRequest {
			typeStr = "Echo Request"
		} else if icmp.TypeCode.Type() == layers.ICMPv4TypeEchoReply {
			typeStr = "Echo Reply"
		} else if icmp.TypeCode.Type() == layers.ICMPv4TypeTimeExceeded {
			typeStr = "Time Exceeded (TTL Expired)"
		}
		summary.Info = fmt.Sprintf("%s (id=%d, seq=%d)", typeStr, icmp.Id, icmp.Seq)
	} else if icmpLayer := packet.Layer(layers.LayerTypeICMPv6); icmpLayer != nil {
		icmp6, _ := icmpLayer.(*layers.ICMPv6)
		summary.Protocol = "ICMPv6"
		switch icmp6.TypeCode.Type() {
		case layers.ICMPv6TypeRouterSolicitation:
			summary.Info = "Router Solicitation"
		case layers.ICMPv6TypeRouterAdvertisement:
			summary.Info = "Router Advertisement"
		case layers.ICMPv6TypeNeighborSolicitation:
			summary.Info = "Neighbor Solicitation"
		case layers.ICMPv6TypeNeighborAdvertisement:
			summary.Info = "Neighbor Advertisement"
		case layers.ICMPv6TypeEchoRequest:
			summary.Info = "Echo Request"
		case layers.ICMPv6TypeEchoReply:
			summary.Info = "Echo Reply"
		default:
			summary.Info = fmt.Sprintf("Control Message (%d)", icmp6.TypeCode.Type())
		}
	}

	// ARP Special handling
	if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		arp, _ := arpLayer.(*layers.ARP)
		summary.Protocol = "ARP"
		summary.Source = models.FormatMAC(arp.SourceHwAddress)
		summary.Destination = models.FormatMAC(arp.DstHwAddress)

		srcIP := net.IP(arp.SourceProtAddress).String()
		dstIP := net.IP(arp.DstProtAddress).String()

		if arp.Operation == layers.ARPRequest {
			summary.Info = fmt.Sprintf("Who has %s? Tell %s", dstIP, srcIP)
		} else {
			summary.Info = fmt.Sprintf("%s is at %s", srcIP, models.FormatMAC(arp.SourceHwAddress))
		}
	}

	return summary
}
