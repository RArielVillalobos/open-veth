package api

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Permitir desde cualquier origen en desarrollo
	},
}

// handleTerminal maneja la conexión WebSocket para la terminal
func (s *Server) handleTerminal(c *gin.Context) {
	nodeName := c.Query("node")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node name is required"})
		return
	}

	// 1. Upgrade de HTTP a WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer ws.Close()

	// 2. Preparar el comando a ejecutar
	// Si es un router, entramos directo a vtysh, si no a bash
	// (Por ahora simplificamos: si el nombre tiene 'router' usamos vtysh, si no bash)
	shell := "bash"
	// Podríamos inspeccionar el nodo para saber su tipo real, pero por ahora:
	// if strings.Contains(nodeName, "router") { shell = "vtysh" }

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		Cmd:          []string{shell},
	}

	// 3. Crear el proceso de ejecución en el contenedor
	ctx := context.Background()
	execID, err := s.manager.GetDockerClient().ContainerExecCreate(ctx, nodeName, execConfig)
	if err != nil {
		log.Printf("Error creating exec: %v", err)
		return
	}

	// 4. Conectarse al proceso (Hijack)
	resp, err := s.manager.GetDockerClient().ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		log.Printf("Error attaching to exec: %v", err)
		return
	}
	defer resp.Close()

	// 5. Puentes de datos (Bi-direccional)

	// Canal de salida: Docker -> WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := resp.Reader.Read(buf)
			if n > 0 {
				if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Canal de entrada: WebSocket -> Docker
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if _, err := resp.Conn.Write(msg); err != nil {
			break
		}
	}
}
