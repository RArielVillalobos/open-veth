package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// GetNodeInterfaces executes 'ip -j addr' inside the container and returns parsed info
func (m *Manager) GetNodeInterfaces(ctx context.Context, containerID string) ([]models.InterfaceInfo, error) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "-j", "addr"},
		AttachStdout: true,
		AttachStderr: true,
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("error attaching to exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return nil, fmt.Errorf("error reading exec output: %v", err)
	}

	if errBuf.Len() > 0 {
		m.logger.Warn("ip addr stderr output", "stderr", errBuf.String())
	}

	var interfaces []models.InterfaceInfo
	if err := json.Unmarshal(outBuf.Bytes(), &interfaces); err != nil {
		return nil, fmt.Errorf("error parsing ip addr json: %v. Output: %s", err, outBuf.String())
	}

	return interfaces, nil
}

// GetNodeRoutes executes 'ip -j route' inside the container
func (m *Manager) GetNodeRoutes(ctx context.Context, containerID string) ([]models.RouteInfo, error) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "-j", "route"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating route exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("error attaching to route exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return nil, err
	}

	var routes []models.RouteInfo
	if err := json.Unmarshal(outBuf.Bytes(), &routes); err != nil {
		return nil, fmt.Errorf("error parsing routes: %v", err)
	}

	// Filter out docker0 routes (internal docker network)
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
	execConfig := container.ExecOptions{
		Cmd:          []string{"bridge", "fdb", "show", "br", "br0"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating fdb exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("error attaching to fdb exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return nil, err
	}

	return parseFdbOutput(outBuf.String()), nil
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
		// Skip multicast/broadcast MACs
		if strings.HasPrefix(mac, "01:") || strings.HasPrefix(mac, "33:33:") || mac == "ff:ff:ff:ff:ff:ff" {
			continue
		}
		// fields[1] == "dev", fields[2] == port
		if fields[1] != "dev" {
			continue
		}
		port := fields[2]
		// Skip entries on br0 itself
		if port == "br0" {
			continue
		}
		// Skip "self" entries — per-interface infrastructure entries, not learned MACs
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
		// Skip "permanent" entries — the port's own MAC registered by the bridge kernel
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

	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "addr", "add", address, "dev", ifaceName},
		AttachStdout: true,
		AttachStderr: true,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create ip addr exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to ip addr exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return fmt.Errorf("failed to read exec output: %v", err)
	}

	inspectResp, err := m.cli.ContainerExecInspect(ctx, execIDResp.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec: %v", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("ip addr add failed: %s", errBuf.String())
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

	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "route", "replace", dst, "via", gateway, "dev", dev},
		AttachStdout: true,
		AttachStderr: true,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create route exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to route exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return fmt.Errorf("failed to read route exec output: %v", err)
	}

	inspectResp, err := m.cli.ContainerExecInspect(ctx, execIDResp.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect route exec: %v", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("ip route add failed: %s", errBuf.String())
	}

	return nil
}
