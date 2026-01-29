package orchestrator

import (
	"context"
	"open-veth/internal/models"
	"testing"
)

// TestCreateAndDeleteNode es un test de integración real.
// Requiere que Docker esté corriendo en la máquina.
func TestCreateAndDeleteNode(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	manager, err := NewManager()
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
