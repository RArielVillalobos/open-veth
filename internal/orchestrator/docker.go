package orchestrator

import (
	"context"
	"fmt"
	"io"

			"open-veth/internal/models"

		

			"github.com/docker/docker/api/types/container"

			"github.com/docker/docker/api/types/image"

		
		"github.com/docker/docker/client"
	)
	
	// Manager se encarga de hablar con el Docker Daemon
	type Manager struct {
		cli *client.Client
	}
	
	// NewManager crea una nueva instancia del orquestador
	func NewManager() (*Manager, error) {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("error al conectar con Docker: %v", err)
		}
	
			return &Manager{cli: cli}, nil
	
		}
	
		
	
		// GetDockerClient devuelve el cliente de Docker interno
	
		func (m *Manager) GetDockerClient() *client.Client {
	
			return m.cli
	
		}
	
		
	
		// CreateNode crea y arranca un contenedor para un nodo de la topología
	
		
	func (m *Manager) CreateNode(ctx context.Context, node models.Node) (string, error) {
		fmt.Printf("Orquestando nodo: %s (Imagen: %s)...\n", node.Name, node.Image)
	
		// 1. Verificar si la imagen existe localmente
		_, _, errInspect := m.cli.ImageInspectWithRaw(ctx, node.Image)
		if errInspect != nil {
			if client.IsErrNotFound(errInspect) {
				// No existe localmente, intentamos Pull
				fmt.Printf("Imagen %s no encontrada localmente, descargando...\n", node.Image)
				reader, errPull := m.cli.ImagePull(ctx, node.Image, image.PullOptions{})
				if errPull != nil {
					return "", fmt.Errorf("error al descargar imagen %s: %v", node.Image, errPull)
				}
				defer reader.Close()
				io.Copy(io.Discard, reader)
			} else {
				// Error desconocido al inspeccionar
				return "", fmt.Errorf("error verificando imagen %s: %v", node.Image, errInspect)
			}
		} else {
			// fmt.Printf("Imagen %s encontrada localmente.\n", node.Image)
		}
	
		// 2. Configuración del contenedor
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
	
		// 3. Crear el contenedor (Manejo de conflictos)
		resp, err := m.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, node.Name)
		if err != nil {
			// Verificamos si el error es porque ya existe
			if client.IsErrNotFound(err) {
				return "", err
			}
			// Si el error contiene "Conflict", asumimos que ya existe y tratamos de recuperarlo
			// Nota: El SDK no exporta un IsErrConflict claro, chequeamos el string o intentamos inspeccionar
			// Una estrategia mejor es intentar inspeccionar primero.
			inspect, inspectErr := m.cli.ContainerInspect(ctx, node.Name)
			if inspectErr == nil {
				fmt.Printf("El nodo %s ya existe (ID: %s). Reutilizando...\n", node.Name, inspect.ID[:12])
				
				// Asegurarnos de que esté corriendo
				if !inspect.State.Running {
					fmt.Printf("El nodo %s estaba detenido. Arrancando...\n", node.Name)
					if errStart := m.cli.ContainerStart(ctx, inspect.ID, container.StartOptions{}); errStart != nil {
						return "", fmt.Errorf("error al arrancar nodo existente: %v", errStart)
					}
				}
				return inspect.ID, nil
			}
			
			// Si no era un conflicto simple, devolvemos el error original
			return "", fmt.Errorf("error al crear contenedor: %v", err)
		}
	
		// 4. Start container
		if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return "", fmt.Errorf("error starting container: %v", err)
		}
	
		// 5. Rename eth0 -> mgmt0 to avoid confusion with lab interfaces
		// We execute 'ip link set dev eth0 name mgmt0' inside the container immediately after start.
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
	
	// DeleteNode detiene y elimina un contenedor (Cleanup)
	func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {
		fmt.Printf("Eliminando nodo %s...\n", nodeName)
		
		// Forzar eliminación (Force=true mata el proceso si está corriendo)
		err := m.cli.ContainerRemove(ctx, nodeName, container.RemoveOptions{
			Force: true, 
		})
		if err != nil {
			if client.IsErrNotFound(err) {
				return nil // Ya no existe, misión cumplida
			}
			return fmt.Errorf("error al eliminar nodo %s: %v", nodeName, err)
		}
		
		return nil
	}
	
	// TestConnection verifica si el daemon de Docker responde
	func (m *Manager) TestConnection(ctx context.Context) error {
		_, err := m.cli.Ping(ctx)
		if err != nil {
			return fmt.Errorf("no se pudo conectar a Docker: %v. ¿Está Docker corriendo?", err)
		}
		fmt.Println("Conexión con Docker establecida con éxito.")
		return nil
	}
	
	// ListNodes muestra los contenedores gestionados por OpenVeth (mock básico)
	func (m *Manager) ListNodes(ctx context.Context) error {
		containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return err
		}
	
		fmt.Printf("Encontrados %d contenedores en el host.\n", len(containers))
		for _, c := range containers {
			fmt.Printf("- %s (ID: %s)\n", c.Names[0], c.ID[:10])
		}
		return nil
	}
			// GetNodePID obtiene el PID del proceso principal del contenedor
		func (m *Manager) GetNodePID(ctx context.Context, containerID string) (int, error) {
			inspect, err := m.cli.ContainerInspect(ctx, containerID)
			if err != nil {
				return 0, fmt.Errorf("error inspeccionando contenedor %s: %v", containerID, err)
			}
			
			if !inspect.State.Running {
				return 0, fmt.Errorf("el contenedor %s no está corriendo", containerID)
			}
		
			return inspect.State.Pid, nil
		}
		
		
