package orchestrator

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"open-veth/internal/models"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// TestCreateAndDeleteNode es un test de integración real.
// Requiere que Docker esté corriendo en la máquina.
func TestCreateAndDeleteNode(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // Silent logger for tests
	manager, err := NewManager(logger)
	if err != nil {
		t.Fatalf("Error inicializando manager: %v", err)
	}

	// Verificar si Docker está vivo antes de intentar nada
	if err := manager.TestConnection(ctx); err != nil {
		t.Skip("Docker no está disponible, saltando test de integración")
	}

	// Definir un nodo de prueba
	testNode := models.Node{
		ID:    "test-unit-id",
		Name:  "test-unit-router", // Nombre único para el test
		Type:  models.ROUTER,
		Image: "alpine:latest",
	}

	// Asegurarnos de limpiar al final (Defer se ejecuta siempre al salir de la función)
	defer func() {
		err := manager.DeleteNode(ctx, testNode.Name)
		if err != nil {
			t.Logf("Error en cleanup: %v", err)
		}
	}()

	// 2. Ejecutar Acción (Crear)
	t.Logf("Intentando crear nodo %s...", testNode.Name)
	containerID, err := manager.CreateNode(ctx, testNode)

	// 3. Aserciones (Verificar resultado)
	if err != nil {
		t.Fatalf("CreateNode falló: %v", err)
	}

	if containerID == "" {
		t.Errorf("Se esperaba un Container ID, se recibió vacío")
	}

	t.Logf("✅ Éxito: Contenedor creado con ID %s", containerID)
}

// TestCreateHubNode verifies that a HUB node can be created with a bridge without MAC learning
func TestCreateHubNode(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager, err := NewManager(logger)
	if err != nil {
		t.Fatalf("Error initializing manager: %v", err)
	}

	// Check if Docker is running
	if err := manager.TestConnection(ctx); err != nil {
		t.Skip("Docker is not available, skipping integration test")
	}

	// Define a test HUB node
	testNode := models.Node{
		ID:    "test-hub-id",
		Name:  "test-unit-hub",
		Type:  models.HUB,
		Image: "alpine:latest",
	}

	// Cleanup
	defer func() {
		err := manager.DeleteNode(ctx, testNode.Name)
		if err != nil {
			t.Logf("Cleanup error: %v", err)
		}
	}()

	// 2. Execute Action (Create)
	t.Logf("Trying to create HUB node %s...", testNode.Name)
	containerID, err := manager.CreateNode(ctx, testNode)

	// 3. Assertions
	if err != nil {
		t.Fatalf("CreateNode failed for HUB: %v", err)
	}

	if containerID == "" {
		t.Errorf("Expected a Container ID, received empty")
	}

	t.Logf("✅ Success: HUB container created with ID %s", containerID)

	// Verify that the bridge exists (this would require running commands inside the container)
	// For now we only verify that creation did not fail
}

// execAndCapture runs a command inside a container and returns its stdout as a string.
func execAndCapture(ctx context.Context, m *Manager, containerID string, cmd []string) (string, error) {
	execID, err := m.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	})
	if err != nil {
		return "", err
	}
	resp, err := m.cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	var out bytes.Buffer
	stdcopy.StdCopy(&out, io.Discard, resp.Reader)
	return out.String(), nil
}

// TestCreateCloudNode verifies that a CLOUD node is created with ip_forward enabled
// and an iptables MASQUERADE rule on eth0.
func TestCreateCloudNode(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager, err := NewManager(logger)
	if err != nil {
		t.Fatalf("Error initializing manager: %v", err)
	}

	if err := manager.TestConnection(ctx); err != nil {
		t.Skip("Docker is not available, skipping integration test")
	}

	testNode := models.Node{
		ID:    "test-cloud-id",
		Name:  "test-unit-cloud",
		Type:  models.CLOUD,
		Image: models.GetImageForType(models.CLOUD),
	}

	defer func() {
		if err := manager.DeleteNode(ctx, testNode.Name); err != nil {
			t.Logf("Cleanup error: %v", err)
		}
	}()

	containerID, err := manager.CreateNode(ctx, testNode)
	if err != nil {
		t.Fatalf("CreateNode failed for CLOUD: %v", err)
	}
	if containerID == "" {
		t.Fatal("Expected a container ID, received empty")
	}
	t.Logf("✅ CLOUD container created with ID %s", containerID)

	// Verify ip_forward = 1
	out, err := execAndCapture(ctx, manager, containerID, []string{"sysctl", "net.ipv4.ip_forward"})
	if err != nil {
		t.Fatalf("Failed to check ip_forward: %v", err)
	}
	if !strings.Contains(out, "net.ipv4.ip_forward = 1") {
		t.Errorf("Expected ip_forward=1, got: %q", out)
	}
	t.Log("✅ ip_forward = 1 confirmed")

	// Verify iptables MASQUERADE rule exists on eth0
	out, err = execAndCapture(ctx, manager, containerID, []string{"iptables", "-t", "nat", "-L", "POSTROUTING", "-v"})
	if err != nil {
		t.Fatalf("Failed to check iptables: %v", err)
	}
	if !strings.Contains(out, "MASQUERADE") {
		t.Errorf("Expected MASQUERADE rule in POSTROUTING, got: %q", out)
	}
	t.Log("✅ iptables MASQUERADE rule confirmed")
}

// TestCreateSwitchNode verifies that a SWITCH node can be created with a normal bridge
func TestCreateSwitchNode(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager, err := NewManager(logger)
	if err != nil {
		t.Fatalf("Error initializing manager: %v", err)
	}

	// Check if Docker is running
	if err := manager.TestConnection(ctx); err != nil {
		t.Skip("Docker is not available, skipping integration test")
	}

	// Define a test SWITCH node
	testNode := models.Node{
		ID:    "test-switch-id",
		Name:  "test-unit-switch",
		Type:  models.SWITCH,
		Image: "alpine:latest",
	}

	// Cleanup
	defer func() {
		err := manager.DeleteNode(ctx, testNode.Name)
		if err != nil {
			t.Logf("Cleanup error: %v", err)
		}
	}()

	// 2. Execute Action (Create)
	t.Logf("Trying to create SWITCH node %s...", testNode.Name)
	containerID, err := manager.CreateNode(ctx, testNode)

	// 3. Assertions
	if err != nil {
		t.Fatalf("CreateNode failed for SWITCH: %v", err)
	}

	if containerID == "" {
		t.Errorf("Expected a Container ID, received empty")
	}

	t.Logf("✅ Success: SWITCH container created with ID %s", containerID)
}
