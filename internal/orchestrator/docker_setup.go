package orchestrator

import (
	"context"
	"fmt"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types/container"
)

// renameMgmtInterface attempts to rename eth0 to docker0. It's safe to call multiple times.
func (m *Manager) renameMgmtInterface(ctx context.Context, containerID, nodeName string) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "link", "set", "dev", "eth0", "name", "docker0"},
		AttachStdout: false,
		AttachStderr: false,
	}

	if execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		_ = m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})
	}
}

// deleteDefaultRoute removes the default route from a container.
// This forces students to configure routing manually in lab exercises.
func (m *Manager) deleteDefaultRoute(ctx context.Context, containerID string) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "route", "del", "default"},
		AttachStdout: false,
		AttachStderr: false,
	}

	if execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		_ = m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})
	}
	// Ignore errors - route might not exist
}

// setupCloud configures a CLOUD node as a NAT gateway.
// It enables IP forwarding and sets up masquerade so connected lab nodes
// can reach the internet through the CLOUD's docker0 (Docker bridge) interface.
// The iptables check (-C) prevents duplicate rules on container restart.
func (m *Manager) setupCloud(ctx context.Context, containerID, nodeName string) {
	setupCmd := "sysctl -w net.ipv4.ip_forward=1 && " +
		"iptables -t nat -C POSTROUTING -o docker0 -j MASQUERADE 2>/dev/null || " +
		"iptables -t nat -A POSTROUTING -o docker0 -j MASQUERADE"

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", setupCmd},
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}

	m.logger.Info("configuring cloud NAT gateway", "name", nodeName)

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		if errStart := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); errStart != nil {
			m.logger.Warn("failed to configure cloud NAT", "name", nodeName, "error", errStart)
		}
	} else {
		m.logger.Warn("failed to create cloud NAT exec", "name", nodeName, "error", err)
	}
}

// setupInternetGateway restores the default gateway on SERVER nodes after eth0→docker0 rename.
// The default route is deleted for all nodes so students configure routing manually,
// but SERVER nodes need it back to reach the internet for apt-get, etc.
func (m *Manager) setupInternetGateway(ctx context.Context, containerID, nodeName string) {
	// Docker's default bridge gateway is always the .1 address of the container's subnet.
	// We detect it by inspecting the container's network settings.
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		m.logger.Warn("setupInternetGateway: failed to inspect container", "name", nodeName, "error", err)
		return
	}

	gateway := inspect.NetworkSettings.Gateway
	if gateway == "" {
		// On custom Docker networks the gateway is inside Networks map, not the top-level field
		for _, network := range inspect.NetworkSettings.Networks {
			if network.Gateway != "" {
				gateway = network.Gateway
				break
			}
		}
	}
	if gateway == "" {
		m.logger.Warn("setupInternetGateway: no gateway found", "name", nodeName)
		return
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "route", "add", "default", "via", gateway, "dev", "docker0"},
		AttachStdout: false,
		AttachStderr: false,
	}

	m.logger.Info("restoring default gateway", "name", nodeName, "gateway", gateway)

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		if err := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			m.logger.Warn("setupInternetGateway: failed to add default route", "name", nodeName, "error", err)
		}
	} else {
		m.logger.Warn("setupInternetGateway: failed to create exec", "name", nodeName, "error", err)
	}
}

// setupBridge initializes the bridge interface (br0) inside a switch or hub container.
// For HUB nodes, ageing_time is set to 0 to disable MAC learning (L1 flood behavior).
// Returns an error if the bridge could not be created — callers must not treat this
// as fire-and-forget, since a failed bridge leaves every attached port isolated.
func (m *Manager) setupBridge(ctx context.Context, containerID, nodeName string, nodeType models.NodeType) error {
	setupCmd := "ip link add name br0 type bridge; ip link set dev br0 up"
	if nodeType == models.HUB {
		setupCmd = "ip link add name br0 type bridge; ip link set dev br0 type bridge ageing_time 0 forward_delay 0; ip link set dev br0 up"
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", setupCmd},
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}

	m.logger.Info("initializing bridge", "name", nodeName, "type", nodeType)

	execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create bridge setup exec: %v", err)
	}
	if err := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start bridge setup: %v", err)
	}
	if inspect, err := m.cli.ContainerExecInspect(ctx, execID.ID); err == nil && inspect.ExitCode != 0 {
		return fmt.Errorf("bridge setup exited with code %d", inspect.ExitCode)
	}
	return nil
}

// AttachInterfaceToBridge connects a network interface to the main bridge (br0) inside a container.
// For HUB nodes, MAC learning is disabled on the interface to simulate L1 flood behavior.
func (m *Manager) AttachInterfaceToBridge(ctx context.Context, containerID string, ifaceName string, nodeType models.NodeType) error {
	if err := models.ValidateInterfaceName(ifaceName); err != nil {
		return fmt.Errorf("bridge attach rejected: %v", err)
	}

	// Step 1: Set interface master to br0
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "link", "set", "dev", ifaceName, "master", "br0"},
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}

	execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec for bridge attach: %v", err)
	}
	if err := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start exec for bridge attach: %v", err)
	}
	if inspect, err := m.cli.ContainerExecInspect(ctx, execID.ID); err == nil && inspect.ExitCode != 0 {
		return fmt.Errorf("attach %s to br0 exited with code %d (is br0 up?)", ifaceName, inspect.ExitCode)
	}

	// Step 2: Configure hub interface: disable MAC learning, explicitly enable flood
	if nodeType == models.HUB {
		execConfigHub := container.ExecOptions{
			Cmd:          []string{"bridge", "link", "set", "dev", ifaceName, "learning", "off", "flood", "on"},
			AttachStdout: true,
			AttachStderr: true,
			Privileged:   true,
		}

		execIDHub, err := m.cli.ContainerExecCreate(ctx, containerID, execConfigHub)
		if err != nil {
			m.logger.Warn("failed to configure hub interface", "iface", ifaceName, "error", err)
		} else {
			if err := m.cli.ContainerExecStart(ctx, execIDHub.ID, container.ExecStartOptions{}); err != nil {
				m.logger.Warn("failed to start hub interface config", "iface", ifaceName, "error", err)
			}
		}
	}

	// Step 3: Bring interface up
	execConfig2 := container.ExecOptions{
		Cmd:          []string{"ip", "link", "set", "dev", ifaceName, "up"},
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}

	execID2, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig2)
	if err != nil {
		return fmt.Errorf("failed to create exec for interface up: %v", err)
	}
	if err := m.cli.ContainerExecStart(ctx, execID2.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start exec for interface up: %v", err)
	}
	if inspect, err := m.cli.ContainerExecInspect(ctx, execID2.ID); err == nil && inspect.ExitCode != 0 {
		return fmt.Errorf("bringing up %s exited with code %d", ifaceName, inspect.ExitCode)
	}

	return nil
}
