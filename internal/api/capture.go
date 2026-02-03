package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"open-veth/internal/models"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

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

// upgrader is already defined in terminal.go

// handleSniff starts a live capture session over WebSockets
func (s *Server) handleSniff(c *gin.Context) {
	nodeID := c.Query("node_id")
	iface := c.Query("interface")

	if nodeID == "" || iface == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id and interface are required"})
		return
	}

	node, found := s.repo.GetNode(nodeID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("Sniff WS Upgrade error: %v\n", err)
		return
	}
	defer ws.Close()

	ctx := c.Request.Context()

	// 1. Prepare tcpdump command
	// -i: interface
	// -w -: write to stdout in binary format
	// -U: unbuffered
	// -s 0: capture full packet
	cmd := []string{"tcpdump", "-i", iface, "-w", "-", "-U", "-s", "0"}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,  // We need stderr to capture errors, but we filter it out
		Tty:          false, // TTY adds formatting, we want raw binary
	}

	execID, err := s.manager.GetDockerClient().ContainerExecCreate(ctx, node.ContainerID, execConfig)
	if err != nil {
		ws.WriteJSON(gin.H{"error": "failed to create sniffer: " + err.Error()})
		return
	}

	// 2. Start the capture stream
	resp, err := s.manager.GetDockerClient().ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		ws.WriteJSON(gin.H{"error": "failed to start sniffer: " + err.Error()})
		return
	}
	defer resp.Close()

	// CLEANUP: Ensure tcpdump is killed when we exit (e.g. client disconnect)
	defer func() {
		// We use the helper to kill the specific tcpdump process on this interface
		pattern := fmt.Sprintf("tcpdump -i %s", iface)
		_ = s.manager.KillProcessByName(context.Background(), node.ContainerID, pattern)
		fmt.Printf("Sniffer cleanup: killed tcpdump on %s (%s)\n", node.Name, iface)
	}()

	// 3. Demultiplex Docker Stream (Header + Payload)
	// We need a pipe: StdCopy writes to PipeWriter -> PipeReader feeds pcapgo
	pr, pw := io.Pipe()

	// Start a goroutine to copy Docker STDOUT to the Pipe
	go func() {
		defer pw.Close()
		// StdCopy demultiplexes execution stream. We discard Stderr.
		_, err := stdcopy.StdCopy(pw, io.Discard, resp.Reader)
		if err != nil {
			fmt.Printf("Sniffer Stream Error: %v\n", err)
		}
	}()

	// 4. Use pcapgo to read from the Pipe (Clean STDOUT)
	pcapReader, err := pcapgo.NewReader(pr)
	if err != nil {
		// Pcap header might be delayed or stream might be empty initially
		fmt.Printf("PCAP Reader Init Error (might be empty stream): %v\n", err)
		ws.WriteJSON(gin.H{"error": "pcap init error: " + err.Error()})
		return
	}

	fmt.Printf("Live capture started on %s (%s)\n", node.Name, iface)

	// 5. Packet processing loop
	for {
		data, ci, err := pcapReader.ReadPacketData()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Don't crash on read errors, just wait or break
			fmt.Printf("Packet read error: %v\n", err)
			break
		}

		// Parse packet for summary
		packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
		summary := s.parsePacket(packet, ci.Timestamp)

		// Send to WebSocket
		if err := ws.WriteJSON(summary); err != nil {
			break // Connection closed by client
		}
	}

	fmt.Printf("Live capture stopped on %s (%s)\n", node.Name, iface)
}

// parsePacket extracts human-readable info from raw packet data

func (s *Server) parsePacket(packet gopacket.Packet, ts time.Time) PacketSummary {

	summary := PacketSummary{

		Timestamp: ts.Format(time.RFC3339), // Send ISO8601 (e.g., 2023-10-10T15:00:00Z)

		Length: len(packet.Data()),

		Protocol: "L2",

		Info: "Ethernet Frame",
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

		summary.Info = fmt.Sprintf("%d → %d [%s] Seq=%d Ack=%d",

			tcp.SrcPort, tcp.DstPort, flags, tcp.Seq, tcp.Ack)

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
