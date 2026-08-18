package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"open-veth/internal/models"
)

// GetNodeInterfaces executes 'ip -j addr' inside the container and returns parsed info
func (m *Manager) GetNodeInterfaces(ctx context.Context, containerID string) ([]models.InterfaceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := m.ExecCommand(ctx, containerID, []string{"ip", "-j", "addr"})
	if err != nil {
		return nil, fmt.Errorf("ip addr: %w", err)
	}

	var interfaces []models.InterfaceInfo
	if err := json.Unmarshal([]byte(out), &interfaces); err != nil {
		return nil, fmt.Errorf("error parsing ip addr json: %v. Output: %s", err, out)
	}
	return interfaces, nil
}

// GetNodeRoutes executes 'ip -j route' inside the container
func (m *Manager) GetNodeRoutes(ctx context.Context, containerID string) ([]models.RouteInfo, error) {
	out, err := m.ExecCommand(ctx, containerID, []string{"ip", "-j", "route"})
	if err != nil {
		return nil, fmt.Errorf("ip route: %w", err)
	}

	var routes []models.RouteInfo
	if err := json.Unmarshal([]byte(out), &routes); err != nil {
		return nil, fmt.Errorf("error parsing routes: %v", err)
	}

	var cleanRoutes []models.RouteInfo
	for _, r := range routes {
		if r.Dev != "docker0" {
			cleanRoutes = append(cleanRoutes, r)
		}
	}
	return cleanRoutes, nil
}

// GetNodeMacTable runs 'bridge fdb show br br0' inside a switch container and returns parsed MAC entries.
func (m *Manager) GetNodeMacTable(ctx context.Context, containerID string) ([]models.MacEntry, error) {
	out, err := m.ExecCommand(ctx, containerID, []string{"bridge", "fdb", "show", "br", "br0"})
	if err != nil {
		return nil, fmt.Errorf("bridge fdb: %w", err)
	}
	return parseFdbOutput(out), nil
}

// parseFdbOutput parses the text output of 'bridge fdb show br br0' into MacEntry slice.
// It skips multicast MACs, self entries, permanent entries, and br0 port entries —
// only dynamic (learned) and explicit static unicast entries are returned.
func parseFdbOutput(output string) []models.MacEntry {
	var entries []models.MacEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mac := fields[0]
		if strings.HasPrefix(mac, "01:") || strings.HasPrefix(mac, "33:33:") || mac == "ff:ff:ff:ff:ff:ff" {
			continue
		}
		if fields[1] != "dev" {
			continue
		}
		port := fields[2]
		if port == "br0" {
			continue
		}
		isSelf := false
		for _, f := range fields[3:] {
			if f == "self" {
				isSelf = true
				break
			}
		}
		if isSelf {
			continue
		}
		entryType := "dynamic"
		isPermanent := false
		for _, f := range fields[3:] {
			if f == "permanent" {
				isPermanent = true
				break
			}
			if f == "static" {
				entryType = "static"
			}
		}
		if isPermanent {
			continue
		}
		entries = append(entries, models.MacEntry{MAC: mac, Port: port, Type: entryType})
	}
	return entries
}

// ConfigureInterface adds an IP address to an interface inside a container
func (m *Manager) ConfigureInterface(ctx context.Context, containerID, ifaceName, address string) error {
	if err := models.ValidateInterfaceName(ifaceName); err != nil {
		return fmt.Errorf("configure interface rejected: %v", err)
	}
	if err := models.ValidateIPCIDR(address); err != nil {
		return fmt.Errorf("configure interface rejected: %v", err)
	}

	if _, err := m.ExecCommand(ctx, containerID, []string{"ip", "addr", "add", address, "dev", ifaceName}); err != nil {
		return fmt.Errorf("ip addr add failed: %w", err)
	}
	return nil
}

// ConfigureRoute adds a static route inside a container
func (m *Manager) ConfigureRoute(ctx context.Context, containerID, dst, gateway, dev string) error {
	if err := models.ValidateInterfaceName(dev); err != nil {
		return fmt.Errorf("configure route rejected: %v", err)
	}
	if err := models.ValidateIPCIDR(dst); err != nil {
		return fmt.Errorf("configure route rejected: %v", err)
	}

	if _, err := m.ExecCommand(ctx, containerID, []string{"ip", "route", "replace", dst, "via", gateway, "dev", dev}); err != nil {
		return fmt.Errorf("ip route add failed: %w", err)
	}
	return nil
}
