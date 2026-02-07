package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

	hostConfig := &container.HostConfig{

		CapAdd: []string{"NET_ADMIN"},
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

			// Ensure Switch Bridge exists (it might be lost on restart)
			if node.Type == models.SWITCH {
				m.setupSwitchBridge(ctx, inspect.ID, node.Name)
			}

			// Ensure Management Interface is renamed (idempotent check)
			m.renameMgmtInterface(ctx, inspect.ID, node.Name)

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
	m.renameMgmtInterface(ctx, resp.ID, node.Name)

	// 7. If node is SWITCH, initialize bridge 'br0'

	if node.Type == models.SWITCH {
		m.setupSwitchBridge(ctx, resp.ID, node.Name)
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

// setupSwitchBridge initializes the bridge interface inside a switch container
func (m *Manager) setupSwitchBridge(ctx context.Context, containerID, nodeName string) {
	// Use specific shell command to ensure bridge creation
	// We use ; instead of && to be safer with simple shells, though && is standard
	setupCmd := []string{"sh", "-c", "ip link add name br0 type bridge; ip link set dev br0 up"}

	execConfigSwitch := container.ExecOptions{
		Cmd:          setupCmd,
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true, // Ensure privileges for netlink ops
	}

	m.logger.Info("initializing bridge for switch", "name", nodeName)

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfigSwitch); err == nil {
		if errStart := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); errStart != nil {
			m.logger.Error("failed to start switch setup", "name", nodeName, "error", errStart)
		}
	} else {
		m.logger.Error("failed to create switch setup exec", "name", nodeName, "error", err)
	}
}

// AttachInterfaceToBridge connects a network interface to the main bridge (br0) inside a container
func (m *Manager) AttachInterfaceToBridge(ctx context.Context, containerID string, ifaceName string) error {
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

	// Step 2: Bring interface up
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
