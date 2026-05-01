package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/pkg/stdcopy"
)

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

// GetOpenVethContainers returns all containers managed by OpenVeth (label openveth=true)
func (m *Manager) GetOpenVethContainers(ctx context.Context) ([]types.Container, error) {
	f := filters.NewArgs()
	f.Add("label", "openveth=true")

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, err
	}
	return containers, nil
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

// RemoveAllOpenVethVolumes removes any legacy openveth-frr-* volumes left from older versions.
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
	execConfig := container.ExecOptions{
		Cmd:          []string{"pkill", "-f", pattern},
		AttachStdout: false,
		AttachStderr: false,
	}

	execIDResp, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create pkill exec: %v", err)
	}

	return m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})
}
