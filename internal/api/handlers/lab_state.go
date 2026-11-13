package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"

	"open-veth/internal/models"
	"open-veth/internal/storage"
)

type labStateReader interface {
	GetNodeInterfaces(context.Context, string) ([]models.InterfaceInfo, error)
	GetNodeRoutes(context.Context, string) ([]models.RouteInfo, error)
}

// captureLabState preserves the previous configuration of every node that cannot
// be read completely. A successful empty read intentionally clears that node.
func captureLabState(ctx context.Context, reader labStateReader, repo storage.Repository, labID string, nodes []models.Node) (
	configs []models.InterfaceConfig, routes []models.RouteConfig, captured bool, err error,
) {
	previousIPs, err := repo.GetInterfaceConfigsByLab(labID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read saved IPs: %w", err)
	}
	previousRoutes, err := repo.GetRouteConfigsByLab(labID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read saved routes: %w", err)
	}
	updated := make(map[string]bool)
	var failures []error
	for _, node := range nodes {
		if node.ContainerID == "" || node.Status != models.NodeStatusRunning {
			continue
		}
		ifaces, readErr := reader.GetNodeInterfaces(ctx, node.ContainerID)
		if readErr != nil {
			failures = append(failures, fmt.Errorf("read interfaces for %s: %w", node.ID, readErr))
			continue
		}
		nodeRoutes, readErr := reader.GetNodeRoutes(ctx, node.ContainerID)
		if readErr != nil {
			failures = append(failures, fmt.Errorf("read routes for %s: %w", node.ID, readErr))
			continue
		}
		updated[node.ID] = true
		captured = true
		for _, iface := range ifaces {
			if iface.Name == "lo" || iface.Name == "docker0" {
				continue
			}
			for _, addr := range iface.IPAddresses {
				if ip := net.ParseIP(addr.Address); ip != nil && ip.IsLinkLocalUnicast() {
					continue
				}
				configs = append(configs, models.InterfaceConfig{LabID: labID, NodeID: node.ID,
					Interface: iface.Name, Address: fmt.Sprintf("%s/%d", addr.Address, addr.Prefix)})
			}
		}
		for _, route := range nodeRoutes {
			if route.Protocol != "kernel" {
				routes = append(routes, models.RouteConfig{LabID: labID, NodeID: node.ID,
					Dst: route.Dst, Gateway: route.Gateway, Dev: route.Dev})
			}
		}
	}
	for _, cfg := range previousIPs {
		if !updated[cfg.NodeID] {
			configs = append(configs, cfg)
		}
	}
	for _, cfg := range previousRoutes {
		if !updated[cfg.NodeID] {
			routes = append(routes, cfg)
		}
	}
	return configs, routes, captured, errors.Join(failures...)
}
