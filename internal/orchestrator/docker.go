package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// Manager handles communication with the Docker Daemon
type Manager struct {
	cli    *client.Client
	logger *slog.Logger
}

// GetNodeInterfaces executes 'ip -j addr' inside the container and returns parsed info

func (m *Manager) GetNodeInterfaces(ctx context.Context, containerID string) ([]models.InterfaceInfo, error) {

	// 1. Create execution configuration

	execConfig := container.ExecOptions{

		Cmd: []string{"ip", "-j", "addr"},

		AttachStdout: true,

		AttachStderr: true,
	}

	// Add timeout to prevent hanging if the container is unresponsive

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)

	defer cancel()

	// 2. Create the execution instance

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)

	if err != nil {

		return nil, fmt.Errorf("error creating exec: %v", err)

	}

	// 3. Attach and execute (Attach gives us the streams)

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})

	if err != nil {

		return nil, fmt.Errorf("error attaching to exec: %v", err)

	}

	defer resp.Close()

	// 4. Read Stdout (separate from Stderr using stdcopy, Docker mixes streams with headers)

	var outBuf, errBuf bytes.Buffer

	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {

		return nil, fmt.Errorf("error reading exec output: %v", err)

	}

	// Log stderr warning if not critical
	if errBuf.Len() > 0 {
		m.logger.Warn("ip addr stderr output", "stderr", errBuf.String())
	}

	// 5. Parse JSON

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

	// Filter out mgmt0 routes (internal docker network)
	var cleanRoutes []models.RouteInfo
	for _, r := range routes {
		if r.Dev != "mgmt0" {
			cleanRoutes = append(cleanRoutes, r)
		}
	}

	return cleanRoutes, nil
}

// NewManager creates a new orchestrator instance
func NewManager(logger *slog.Logger) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error connecting to Docker: %v", err)
	}

	return &Manager{cli: cli, logger: logger}, nil
}

// GetDockerClient returns the internal Docker client

func (m *Manager) GetDockerClient() *client.Client {

	return m.cli

}

// CreateNode creates and starts a container for a topology node
func (m *Manager) CreateNode(ctx context.Context, node models.Node) (string, error) {
	m.logger.Info("orchestrating node", "name", node.Name, "image", node.Image)

	// 1. Check if image exists locally
	_, _, errInspect := m.cli.ImageInspectWithRaw(ctx, node.Image)
	if errInspect != nil {
		if client.IsErrNotFound(errInspect) {
			// Pull image if not found locally
			m.logger.Info("image not found locally, pulling", "image", node.Image)
			reader, errPull := m.cli.ImagePull(ctx, node.Image, image.PullOptions{})
			if errPull != nil {
				return "", fmt.Errorf("error pulling image %s: %v", node.Image, errPull)
			}
			defer reader.Close()
			io.Copy(io.Discard, reader)
		} else {
			return "", fmt.Errorf("error inspecting image %s: %v", node.Image, errInspect)
		}
	}

	// 2. Container configuration

	config := &container.Config{

		Image: node.Image,

		Hostname: node.Name, // Set hostname to node name (e.g. ROUTER-1)

		Labels: map[string]string{

			"openveth": "true",

			"openveth.name": node.Name,

			"openveth.lab": node.LabID,
		},

		Env: []string{
			"PS1=" + node.Name + ":\\w\\$ ", // Forces the prompt to show node name
		},
	}

	caps := []string{"NET_ADMIN"}
	if node.Type == models.SERVER || node.Type == models.MONITOR {
		// systemd requires SYS_ADMIN to manage cgroups inside the container
		caps = append(caps, "SYS_ADMIN")
	}

	hostConfig := &container.HostConfig{
		CapAdd: caps,
	}

	// Mount named volume for FRR config persistence on routers
	if node.Type == models.ROUTER {
		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: frrVolumeName(node.Name),
				Target: "/etc/frr",
			},
		}
	}

	// systemd requires /sys/fs/cgroup mounted from the host and cgroupns=host
	if node.Type == models.SERVER || node.Type == models.MONITOR {
		hostConfig.CgroupnsMode = "host"
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   "/sys/fs/cgroup",
			Target:   "/sys/fs/cgroup",
			ReadOnly: false,
		})
	}

	// MONITOR: expose Grafana (3000) and Prometheus (9090) on dynamic host ports
	if node.Type == models.MONITOR {
		config.ExposedPorts = nat.PortSet{
			"3000/tcp": struct{}{},
			"9090/tcp": struct{}{},
		}
		hostConfig.PortBindings = nat.PortMap{
			"3000/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
			"9090/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
		}
	}

	// 3. Create container (Conflict handling)

	resp, err := m.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, node.Name)

	if err != nil {

		if client.IsErrNotFound(err) {

			return "", err

		}

		// If container exists, try to recover it
		inspect, inspectErr := m.cli.ContainerInspect(ctx, node.Name)
		if inspectErr == nil {
			m.logger.Info("node already exists, reusing", "name", node.Name, "id", inspect.ID[:12])

			// Ensure it's running
			if !inspect.State.Running {
				m.logger.Info("node was stopped, starting", "name", node.Name)
				if errStart := m.cli.ContainerStart(ctx, inspect.ID, container.StartOptions{}); errStart != nil {
					return "", fmt.Errorf("error starting existing node: %v", errStart)
				}
			}

			// Wait for network stack to be ready
			if err := m.WaitForReady(ctx, inspect.ID); err != nil {
				m.logger.Warn("container not ready after start", "name", node.Name, "error", err)
			}

			// Ensure bridge exists (it might be lost on restart)
			if models.NeedsBridge(node.Type) {
				m.setupBridge(ctx, inspect.ID, node.Name, node.Type)
			}

			// Ensure Management Interface is renamed (idempotent check)
			// CLOUD nodes keep eth0 to maintain Docker bridge connectivity
			if !models.KeepEth0(node.Type) {
				m.renameMgmtInterface(ctx, inspect.ID, node.Name)
				m.deleteDefaultRoute(ctx, inspect.ID)
			}

			// Re-apply NAT rules (lost on container restart)
			if node.Type == models.CLOUD {
				m.setupCloud(ctx, inspect.ID, node.Name)
			}

			// Re-apply default gateway for LINUX/SERVER nodes (lost on container restart)
			if node.Type == models.LINUX || node.Type == models.SERVER {
				m.setupLinuxGateway(ctx, inspect.ID, node.Name)
			}

			return inspect.ID, nil

		}

		return "", fmt.Errorf("error creating container: %v", err)

	}

	// 4. Start container

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {

		return "", fmt.Errorf("error starting container: %v", err)

	}

	// 5. Wait for network stack to be ready
	if err := m.WaitForReady(ctx, resp.ID); err != nil {
		m.logger.Warn("container not ready after start", "name", node.Name, "error", err)
	}

	// 6. Rename eth0 -> mgmt0 to avoid confusion with lab interfaces
	// CLOUD nodes keep eth0 to maintain Docker bridge connectivity (internet access)
	if !models.KeepEth0(node.Type) {
		m.renameMgmtInterface(ctx, resp.ID, node.Name)
		m.deleteDefaultRoute(ctx, resp.ID)
	}

	// 7. If node needs a bridge, initialize 'br0'
	if models.NeedsBridge(node.Type) {
		m.setupBridge(ctx, resp.ID, node.Name, node.Type)
	}

	// 8. If node is CLOUD, enable IP forwarding and NAT masquerade
	if node.Type == models.CLOUD {
		m.setupCloud(ctx, resp.ID, node.Name)
	}

	// 9. If node is LINUX or SERVER, restore default gateway for direct internet access
	if node.Type == models.LINUX || node.Type == models.SERVER {
		m.setupLinuxGateway(ctx, resp.ID, node.Name)
	}

	m.logger.Info("node created and started", "name", node.Name, "id", resp.ID[:12])

	return resp.ID, nil

}

// WaitForReady polls the container until its network stack is ready (loopback is up).
// It retries up to 5 times with 500ms between attempts.
func (m *Manager) WaitForReady(ctx context.Context, containerID string) error {
	for i := 0; i < 5; i++ {
		execConfig := container.ExecOptions{
			Cmd: []string{"ip", "link", "show", "lo"},
		}
		execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
		if err == nil {
			if err := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err == nil {
				inspect, err := m.cli.ContainerExecInspect(ctx, execID.ID)
				if err == nil && inspect.ExitCode == 0 {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return fmt.Errorf("container %s not ready after 2.5s", shortID)
}

// renameMgmtInterface attempts to rename eth0 to mgmt0. It's safe to call multiple times.
func (m *Manager) renameMgmtInterface(ctx context.Context, containerID, nodeName string) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "link", "set", "dev", "eth0", "name", "mgmt0"},
		AttachStdout: false,
		AttachStderr: false,
	}

	if execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		_ = m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})
	} else {
		// This might fail if eth0 doesn't exist (already renamed), which is fine.
		// fmt.Printf("Debug: Renaming eth0->mgmt0 attempt on %s: %v\n", nodeName, err)
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
// can reach the internet through the CLOUD's eth0 (Docker bridge) interface.
// The iptables check (-C) prevents duplicate rules on container restart.
func (m *Manager) setupCloud(ctx context.Context, containerID, nodeName string) {
	setupCmd := "sysctl -w net.ipv4.ip_forward=1 && " +
		"iptables -t nat -C POSTROUTING -o eth0 -j MASQUERADE 2>/dev/null || " +
		"iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE"

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

// setupLinuxGateway restores the default gateway on LINUX nodes after eth0→mgmt0 rename.
// The default route is deleted for all non-CLOUD nodes so students configure routing manually,
// but LINUX nodes need it back to reach the internet for apt-get, pip, etc.
func (m *Manager) setupLinuxGateway(ctx context.Context, containerID, nodeName string) {
	// Docker's default bridge gateway is always the .1 address of the container's subnet.
	// We detect it by inspecting the container's network settings.
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		m.logger.Warn("setupLinuxGateway: failed to inspect container", "name", nodeName, "error", err)
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
		m.logger.Warn("setupLinuxGateway: no gateway found", "name", nodeName)
		return
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"ip", "route", "add", "default", "via", gateway, "dev", "mgmt0"},
		AttachStdout: false,
		AttachStderr: false,
	}

	m.logger.Info("restoring default gateway for linux node", "name", nodeName, "gateway", gateway)

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		if err := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
			m.logger.Warn("setupLinuxGateway: failed to add default route", "name", nodeName, "error", err)
		}
	} else {
		m.logger.Warn("setupLinuxGateway: failed to create exec", "name", nodeName, "error", err)
	}
}

// setupBridge initializes the bridge interface (br0) inside a switch or hub container.
// For HUB nodes, ageing_time is set to 0 to disable MAC learning (L1 flood behavior).
func (m *Manager) setupBridge(ctx context.Context, containerID, nodeName string, nodeType models.NodeType) {
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

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig); err == nil {
		if errStart := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); errStart != nil {
			m.logger.Error("failed to start bridge setup", "name", nodeName, "error", errStart)
		}
	} else {
		m.logger.Error("failed to create bridge setup exec", "name", nodeName, "error", err)
	}
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

	return nil
}

// ContainerEvent represents a relevant Docker container lifecycle event
type ContainerEvent struct {
	Action      string // "die", "start"
	ContainerID string
}

// WatchEvents streams Docker container events filtered to OpenVeth containers.
// It calls the provided callback for each relevant event until ctx is cancelled.
func (m *Manager) WatchEvents(ctx context.Context, callback func(ContainerEvent)) {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("event", "die")
	f.Add("event", "start")
	f.Add("label", "openveth=true")

	eventsCh, errCh := m.cli.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil && ctx.Err() == nil {
				m.logger.Warn("docker event stream error, restarting", "error", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
				eventsCh, errCh = m.cli.Events(ctx, events.ListOptions{Filters: f})
			}
		case ev := <-eventsCh:
			callback(ContainerEvent{
				Action:      string(ev.Action),
				ContainerID: ev.Actor.ID,
			})
		}
	}
}

// StopNode stops a running container without removing it
func (m *Manager) StopNode(ctx context.Context, nodeName string) error {
	m.logger.Info("stopping node", "name", nodeName)
	err := m.cli.ContainerStop(ctx, nodeName, container.StopOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return fmt.Errorf("error stopping node %s: %v", nodeName, err)
	}
	return nil
}

// StartNode starts a stopped container
func (m *Manager) StartNode(ctx context.Context, nodeName string) error {
	m.logger.Info("starting node", "name", nodeName)
	err := m.cli.ContainerStart(ctx, nodeName, container.StartOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			return fmt.Errorf("container %s not found", nodeName)
		}
		return fmt.Errorf("error starting node %s: %v", nodeName, err)
	}
	return nil
}

// DeleteNode stops and removes a container (Cleanup)
func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {
	m.logger.Info("deleting node", "name", nodeName)

	// Force removal (kills process if running)

	err := m.cli.ContainerRemove(ctx, nodeName, container.RemoveOptions{

		Force: true,
	})

	if err != nil {

		if client.IsErrNotFound(err) {

			return nil // Already gone

		}

		return fmt.Errorf("error deleting node %s: %v", nodeName, err)

	}

	return nil

}

// TestConnection checks if Docker daemon is responsive
func (m *Manager) TestConnection(ctx context.Context) error {
	_, err := m.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("could not connect to Docker: %v. Is Docker running?", err)
	}

	m.logger.Info("docker connection established")
	return nil
}

// ListNodes displays containers managed by OpenVeth
func (m *Manager) ListNodes(ctx context.Context) error {
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return err
	}

	m.logger.Info("containers found", "count", len(containers))
	for _, c := range containers {
		m.logger.Debug("container", "name", c.Names[0], "id", c.ID[:10])
	}

	return nil
}

// GetOpenVethContainers returns all containers managed by OpenVeth (label openveth=true)
func (m *Manager) GetOpenVethContainers(ctx context.Context) ([]types.Container, error) {
	filters := filters.NewArgs()
	filters.Add("label", "openveth=true")

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	return containers, nil
}

// GetNodePID gets the main process PID of a container

func (m *Manager) GetNodePID(ctx context.Context, containerID string) (int, error) {

	inspect, err := m.cli.ContainerInspect(ctx, containerID)

	if err != nil {

		return 0, fmt.Errorf("error inspecting container %s: %v", containerID, err)

	}

	if !inspect.State.Running {

		return 0, fmt.Errorf("container %s is not running", containerID)

	}

	return inspect.State.Pid, nil

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

	// Check exec exit code
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

	cmd := []string{"ip", "route", "add", dst, "via", gateway, "dev", dev}
	execConfig := container.ExecOptions{
		Cmd:          cmd,
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

// GetServicePorts returns the host-mapped ports for a MONITOR node.
// Docker assigns random host ports when the container is created with HostPort: "0".
func (m *Manager) GetServicePorts(ctx context.Context, containerID string) (map[string]int, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		shortID := containerID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		return nil, fmt.Errorf("error inspecting container %s: %v", shortID, err)
	}

	ports := make(map[string]int)
	for containerPort, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) == 0 {
			continue
		}
		hostPort, err := strconv.Atoi(bindings[0].HostPort)
		if err != nil || hostPort == 0 {
			continue
		}
		switch containerPort.Port() {
		case "3000":
			ports["grafana"] = hostPort
		case "9090":
			ports["prometheus"] = hostPort
		}
	}
	return ports, nil
}

// frrVolumeName returns the deterministic volume name for a router node's FRR config
func frrVolumeName(nodeName string) string {
	return "openveth-frr-" + nodeName
}

// RemoveNodeVolumes removes any named volumes associated with a node
func (m *Manager) RemoveNodeVolumes(ctx context.Context, nodeName string) {
	volName := frrVolumeName(nodeName)
	if err := m.cli.VolumeRemove(ctx, volName, true); err != nil {
		// Ignore errors — volume may not exist (host/switch nodes)
		m.logger.Debug("volume removal skipped", "volume", volName, "reason", err)
	}
}

// RemoveAllOpenVethVolumes removes all volumes with the openveth-frr- prefix
func (m *Manager) RemoveAllOpenVethVolumes(ctx context.Context) int {
	f := filters.NewArgs()
	f.Add("name", "openveth-frr-")
	listResp, err := m.cli.VolumeList(ctx, volume.ListOptions{Filters: f})
	if err != nil {
		m.logger.Warn("failed to list volumes for cleanup", "error", err)
		return 0
	}
	removed := 0
	for _, v := range listResp.Volumes {
		if err := m.cli.VolumeRemove(ctx, v.Name, true); err != nil {
			m.logger.Warn("failed to remove volume", "volume", v.Name, "error", err)
		} else {
			removed++
		}
	}
	return removed
}

// RunTraceroute executes 'traceroute -n -w 2 -m 15 <dest>' inside the container and returns raw output
func (m *Manager) RunTraceroute(ctx context.Context, containerID string, destination string) (string, error) {
	execConfig := container.ExecOptions{
		Cmd:          []string{"traceroute", "-n", "-w", "2", "-m", "15", destination},
		AttachStdout: true,
		AttachStderr: true,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("error creating traceroute exec: %v", err)
	}

	resp, err := m.cli.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("error attaching to traceroute exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		return "", fmt.Errorf("error reading traceroute output: %v", err)
	}

	return outBuf.String(), nil
}

// KillProcessByName finds processes matching a name pattern inside a container and kills them
func (m *Manager) KillProcessByName(ctx context.Context, containerID, pattern string) error {
	// We use pkill -f to match the full command line
	cmd := []string{"pkill", "-f", pattern}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: false,
		AttachStderr: false,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create pkill exec: %v", err)
	}

	return m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})
}
