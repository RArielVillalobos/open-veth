package orchestrator

import (
	"context"
	"fmt"
	"io"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

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
		Image:    node.Image,
		Hostname: node.Name,
		Labels: map[string]string{
			"openveth":      "true",
			"openveth.name": node.Name,
			"openveth.lab":  node.LabID,
		},
		Env: []string{
			"PS1=" + node.Name + ":\\w\\$ ",
		},
	}

	caps := []string{"NET_ADMIN"}
	if node.Type == models.SERVER || node.Type == models.MONITOR || node.Type == models.STORAGE {
		// systemd requires SYS_ADMIN to manage cgroups inside the container
		caps = append(caps, "SYS_ADMIN")
	}
	if node.Type == models.STORAGE {
		// CAP_MKNOD lets storage-init.service create /dev/loopN nodes at startup
		caps = append(caps, "MKNOD")
	}

	hostConfig := &container.HostConfig{
		CapAdd: caps,
	}

	// systemd nodes (SERVER, MONITOR, STORAGE) need:
	// - seccomp/apparmor unconfined: service sandbox directives use syscalls blocked by Docker defaults
	// - cgroupns=host + /sys/fs/cgroup bind + /run tmpfs: required for systemd as PID 1
	if node.Type == models.SERVER || node.Type == models.MONITOR || node.Type == models.STORAGE {
		hostConfig.SecurityOpt = []string{"seccomp=unconfined", "apparmor=unconfined"}
		hostConfig.CgroupnsMode = "host"
		hostConfig.Mounts = append(hostConfig.Mounts,
			mount.Mount{Type: mount.TypeBind, Source: "/sys/fs/cgroup", Target: "/sys/fs/cgroup"},
			mount.Mount{Type: mount.TypeTmpfs, Target: "/run"},
			mount.Mount{Type: mount.TypeTmpfs, Target: "/run/lock"},
		)
	}

	// STORAGE: expose only loop-control and device-mapper control.
	// The storage-init.service inside the container uses LOOP_CTL_GET_FREE to find
	// the first kernel-free slot and mknod's 8 loop devices starting there —
	// so they never collide with host loop devices already in use (e.g. snapd).
	// /dev/mapper/control is required for LVM and LUKS (device-mapper).
	if node.Type == models.STORAGE {
		hostConfig.Devices = append(hostConfig.Devices,
			container.DeviceMapping{PathOnHost: "/dev/loop-control", PathInContainer: "/dev/loop-control", CgroupPermissions: "rwm"},
			container.DeviceMapping{PathOnHost: "/dev/mapper/control", PathInContainer: "/dev/mapper/control", CgroupPermissions: "rwm"},
		)
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

	// HAPROXY: expose stats page (8404) on a dynamic host port
	if node.Type == models.HAPROXY {
		config.ExposedPorts = nat.PortSet{
			"8404/tcp": struct{}{},
		}
		hostConfig.PortBindings = nat.PortMap{
			"8404/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
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

			if !inspect.State.Running {
				m.logger.Info("node was stopped, starting", "name", node.Name)
				if errStart := m.cli.ContainerStart(ctx, inspect.ID, container.StartOptions{}); errStart != nil {
					return "", fmt.Errorf("error starting existing node: %v", errStart)
				}
			}

			if err := m.WaitForReady(ctx, inspect.ID); err != nil {
				m.logger.Warn("container not ready after start", "name", node.Name, "error", err)
			}

			if models.NeedsBridge(node.Type) {
				if err := m.setupBridge(ctx, inspect.ID, node.Name, node.Type); err != nil {
					m.logger.Error("bridge setup failed", "name", node.Name, "error", err)
				}
			}

			m.renameMgmtInterface(ctx, inspect.ID, node.Name)
			m.deleteDefaultRoute(ctx, inspect.ID)

			if node.Type == models.CLOUD {
				m.setupCloud(ctx, inspect.ID, node.Name)
			}
			if node.Type == models.SERVER || node.Type == models.STORAGE {
				m.setupInternetGateway(ctx, inspect.ID, node.Name)
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

	// 6. Rename eth0 -> docker0 to free eth0 for lab data interfaces
	m.renameMgmtInterface(ctx, resp.ID, node.Name)
	m.deleteDefaultRoute(ctx, resp.ID)

	// 7. If node needs a bridge, initialize 'br0'
	if models.NeedsBridge(node.Type) {
		if err := m.setupBridge(ctx, resp.ID, node.Name, node.Type); err != nil {
			m.logger.Error("bridge setup failed", "name", node.Name, "error", err)
		}
	}

	// 8. If node is CLOUD, enable IP forwarding and NAT masquerade
	if node.Type == models.CLOUD {
		m.setupCloud(ctx, resp.ID, node.Name)
	}

	// 9. If node is SERVER or STORAGE, restore default gateway for direct internet access
	if node.Type == models.SERVER || node.Type == models.STORAGE {
		m.setupInternetGateway(ctx, resp.ID, node.Name)
	}

	m.logger.Info("node created and started", "name", node.Name, "id", resp.ID[:12])

	return resp.ID, nil
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

// DeleteNode stops and removes a container
func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {
	m.logger.Info("deleting node", "name", nodeName)
	err := m.cli.ContainerRemove(ctx, nodeName, container.RemoveOptions{Force: true})
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting node %s: %v", nodeName, err)
	}
	return nil
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

// WaitForHTTP polls a port inside the container with curl until it returns HTTP 2xx,
// or until the timeout elapses. Interval between attempts is 5 seconds.
func (m *Manager) WaitForHTTP(ctx context.Context, containerID string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		execConfig := container.ExecOptions{
			Cmd:          []string{"curl", "-sf", "--max-time", "3", "-o", "/dev/null", fmt.Sprintf("http://localhost:%d", port)},
			AttachStdout: false,
			AttachStderr: false,
		}
		execID, err := m.cli.ContainerExecCreate(ctx, containerID, execConfig)
		if err == nil {
			resp, err := m.cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
			if err == nil {
				resp.Close()
				inspect, err := m.cli.ContainerExecInspect(ctx, execID.ID)
				if err == nil && inspect.ExitCode == 0 {
					return nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("service on port %d not ready after %v", port, timeout)
}
