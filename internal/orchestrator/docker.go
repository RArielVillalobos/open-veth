package orchestrator

import (

	"context"

	"fmt"

	"io"

	"bytes"

	"encoding/json"

	"time"



	"open-veth/internal/models"



	"github.com/docker/docker/api/types/container"

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

		Cmd:          []string{"ip", "-j", "addr"},

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

		Cmd:   []string{"sleep", "infinity"},

		Labels: map[string]string{

			"openveth":      "true",

			"openveth.name": node.Name,

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

			return inspect.ID, nil

		}



		return "", fmt.Errorf("error creating container: %v", err)

	}



	// 4. Start container

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {

		return "", fmt.Errorf("error starting container: %v", err)

	}



	// 5. Rename eth0 -> mgmt0 to avoid confusion with lab interfaces

	execConfig := container.ExecOptions{

		Cmd:          []string{"ip", "link", "set", "dev", "eth0", "name", "mgmt0"},

		AttachStdout: false,

		AttachStderr: false,

	}



	if execIDResp, err := m.cli.ContainerExecCreate(ctx, resp.ID, execConfig); err == nil {

		_ = m.cli.ContainerExecStart(ctx, execIDResp.ID, container.ExecStartOptions{})

	} else {

		fmt.Printf("Warning: Could not rename eth0 to mgmt0 in %s: %v\n", node.Name, err)

	}



	fmt.Printf("Node %s created and started successfully (ID: %s).\n", node.Name, resp.ID[:12])

	return resp.ID, nil

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
		
		
