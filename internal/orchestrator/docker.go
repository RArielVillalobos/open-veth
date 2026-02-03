package orchestrator

import (
	"context"

	"fmt"

	"io"

	"bytes"

	"encoding/json"

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
	cli *client.Client
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

		fmt.Printf("Warning: 'ip -j addr' stderr: %s\n", errBuf.String())

	}

	// 5. Parse JSON

	var interfaces []models.InterfaceInfo

	if err := json.Unmarshal(outBuf.Bytes(), &interfaces); err != nil {

		return nil, fmt.Errorf("error parsing ip addr json: %v. Output: %s", err, outBuf.String())

	}

	return interfaces, nil

}

// NewManager creates a new orchestrator instance

func NewManager() (*Manager, error) {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {

		return nil, fmt.Errorf("error connecting to Docker: %v", err)

	}

	return &Manager{cli: cli}, nil

}

// GetDockerClient returns the internal Docker client

func (m *Manager) GetDockerClient() *client.Client {

	return m.cli

}

// CreateNode creates and starts a container for a topology node

func (m *Manager) CreateNode(ctx context.Context, node models.Node) (string, error) {

	fmt.Printf("Orchestrating node: %s (Image: %s)...\n", node.Name, node.Image)

	// 1. Check if image exists locally

	_, _, errInspect := m.cli.ImageInspectWithRaw(ctx, node.Image)

	if errInspect != nil {

		if client.IsErrNotFound(errInspect) {

			// Pull image if not found locally

			fmt.Printf("Image %s not found locally, pulling...\n", node.Image)

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

		Cmd: []string{"sleep", "infinity"},

		Labels: map[string]string{

			"openveth": "true",

			"openveth.name": node.Name,

			"openveth.lab": node.LabID,
		},
	}

	hostConfig := &container.HostConfig{

		CapAdd: []string{"NET_ADMIN", "SYS_ADMIN"},
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

			fmt.Printf("Node %s already exists (ID: %s). Reusing...\n", node.Name, inspect.ID[:12])

			// Ensure it's running

			if !inspect.State.Running {

				fmt.Printf("Node %s was stopped. Starting...\n", node.Name)

				if errStart := m.cli.ContainerStart(ctx, inspect.ID, container.StartOptions{}); errStart != nil {

					return "", fmt.Errorf("error starting existing node: %v", errStart)

				}

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

	// 5. Rename eth0 -> mgmt0 to avoid confusion with lab interfaces
	m.renameMgmtInterface(ctx, resp.ID, node.Name)

	// 6. If node is SWITCH, initialize bridge 'br0'

	if node.Type == models.SWITCH {
		m.setupSwitchBridge(ctx, resp.ID, node.Name)
	}

	fmt.Printf("Node %s created and started successfully (ID: %s).\n", node.Name, resp.ID[:12])

	return resp.ID, nil

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

	fmt.Printf("Initializing Bridge br0 for Switch %s...\n", nodeName)

	if execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfigSwitch); err == nil {
		if errStart := m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); errStart != nil {
			fmt.Printf("Error starting switch setup: %v\n", errStart)
		}
	} else {
		fmt.Printf("Error creating switch setup exec: %v\n", err)
	}
}

// AttachInterfaceToBridge connects a network interface to the main bridge (br0) inside a container

func (m *Manager) AttachInterfaceToBridge(ctx context.Context, containerID string, ifaceName string) error {

	cmd := fmt.Sprintf("ip link set dev %s master br0 && ip link set dev %s up", ifaceName, ifaceName)

	execConfig := container.ExecOptions{

		Cmd: []string{"sh", "-c", cmd},

		AttachStdout: true,

		AttachStderr: true,

		Privileged: true,
	}

	execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)

	if err != nil {

		return fmt.Errorf("failed to create exec for bridge attach: %v", err)

	}

	err = m.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{})

	if err != nil {

		return fmt.Errorf("failed to start exec for bridge attach: %v", err)

	}

	return nil

}

// DeleteNode stops and removes a container (Cleanup)

func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {

	fmt.Printf("Deleting node %s...\n", nodeName)

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

	fmt.Println("Docker connection established successfully.")

	return nil

}

// ListNodes displays containers managed by OpenVeth

func (m *Manager) ListNodes(ctx context.Context) error {

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})

	if err != nil {

		return err

	}

	fmt.Printf("Found %d containers on host.\n", len(containers))

	for _, c := range containers {

		fmt.Printf("- %s (ID: %s)\n", c.Names[0], c.ID[:10])

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
