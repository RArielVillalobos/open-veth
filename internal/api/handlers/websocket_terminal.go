package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"open-veth/internal/models"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow any origin in development
	},
}

// HandleTerminal manages WebSocket connection for terminal access
func (h *Handler) HandleTerminal(c *gin.Context) {
	nodeName := c.Query("node")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node name is required"})
		return
	}

	// SECURITY: Verify the node exists in our repository before allowing terminal access.
	node, found := h.Repo.GetNode(nodeName)
	if !found {
		// Also try lookup by name
		nodes, err := h.Repo.ListNodes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify node"})
			return
		}

		// Search for the node, prioritizing the one that is running if there are duplicates
		var candidates []models.Node
		for _, n := range nodes {
			if n.Name == nodeName || n.ID == nodeName {
				candidates = append(candidates, n)
			}
		}

		if len(candidates) > 0 {
			found = true
			node = candidates[0]
			// If we have multiple, try to find the one with a container_id
			for _, candidate := range candidates {
				h.hydrateNode(&candidate)
				if candidate.ContainerID != "" {
					node = candidate
					break
				}
			}
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	h.hydrateNode(&node)
	if node.ContainerID == "" {
		h.Logger.Warn("terminal request for stopped node", "name", node.Name, "id", node.ID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
		return
	}

	// 1. Upgrade HTTP to WebSocket
	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.Logger.Error("failed to upgrade to websocket", "error", err)
		return
	}
	defer ws.Close()

	// 2. Prepare command (standard bash for all nodes)
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		Cmd:          []string{"bash"},
		ConsoleSize:  &[2]uint{50, 220}, // default — frontend will send resize immediately after connect
	}

	// 3. Create exec instance in container
	ctx := c.Request.Context()
	execID, err := h.Manager.GetDockerClient().ContainerExecCreate(ctx, node.ContainerID, execConfig)
	if err != nil {
		h.Logger.Error("failed to create exec", "node", node.Name, "error", err)
		return
	}

	// 4. Attach to exec (Hijack)
	resp, err := h.Manager.GetDockerClient().ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		h.Logger.Error("failed to attach to exec", "node", node.Name, "error", err)
		return
	}
	defer resp.Close()

	h.Logger.Info("terminal session started", "node", node.Name)

	// 5. Bidirectional data bridge

	// Buffer to accumulate terminal input
	var inputBuffer strings.Builder

	// Output: Docker -> WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := resp.Reader.Read(buf)
			if n > 0 {
				if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Input: WebSocket -> Docker
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}

		// Handle terminal resize control message: {"type":"resize","cols":N,"rows":N}
		if cols, rows, ok := parseResizeMessage(msg); ok {
			h.Manager.GetDockerClient().ContainerExecResize(ctx, execID.ID, container.ResizeOptions{ //nolint:errcheck
				Width:  cols,
				Height: rows,
			})
			continue
		}

		cmd := string(msg)

		// Handle special keys and control characters
		if len(cmd) == 1 {
			switch cmd[0] {
			case '\r', '\n':
				// Enter pressed - process the accumulated command
				fullCmd := strings.TrimSpace(inputBuffer.String())
				inputBuffer.Reset()

				if fullCmd != "" {
					h.Logger.Debug("terminal command executed", "node", node.Name, "cmd", fullCmd)

					if isNetworkConfigCommand(fullCmd) {
						h.Logger.Debug("network config command detected", "node", node.Name, "cmd", fullCmd)
						go func(nodeID, labID string) {
							// Wait for command to execute, then broadcast interface change
							time.Sleep(1 * time.Second)
							h.BroadcastInterfaceChanged(nodeID, labID)
						}(node.ID, node.LabID)
					}
				}

			case '\b', 0x7F:
				// Backspace - remove last character
				current := inputBuffer.String()
				if len(current) > 0 {
					inputBuffer.Reset()
					inputBuffer.WriteString(current[:len(current)-1])
				}

			case 0x03, 0x04:
				// Ctrl+C or Ctrl+D - clear buffer
				inputBuffer.Reset()

			default:
				// Only accept printable ASCII characters (32-126)
				if cmd[0] >= 32 && cmd[0] <= 126 {
					inputBuffer.WriteString(cmd)
				}
			}
		} else if len(cmd) > 1 {
			// Multi-character input (paste, escape sequences, etc.)
			// Strip ANSI escape sequences (e.g. bracketed paste markers)
			cleaned := stripEscapeSequences(cmd)
			for _, ch := range cleaned {
				switch {
				case ch == '\r' || ch == '\n':
					// Enter found in pasted text - process accumulated command
					fullCmd := strings.TrimSpace(inputBuffer.String())
					inputBuffer.Reset()

					if fullCmd != "" {
						h.Logger.Debug("terminal command executed (paste)", "node", node.Name, "cmd", fullCmd)

						if isNetworkConfigCommand(fullCmd) {
							h.Logger.Debug("network config command detected", "node", node.Name, "cmd", fullCmd)
							go func(nodeID, labID string) {
								time.Sleep(1 * time.Second)
								h.BroadcastInterfaceChanged(nodeID, labID)
							}(node.ID, node.LabID)
						}
					}
				case ch >= 32 && ch <= 126:
					inputBuffer.WriteRune(ch)
				}
			}
		}

		if _, err := resp.Conn.Write(msg); err != nil {
			break
		}
	}

	h.Logger.Info("terminal session ended", "node", node.Name)
}

// stripEscapeSequences removes ANSI escape sequences (CSI) from input.
// This handles bracketed paste markers (\x1b[200~ / \x1b[201~) and other
// terminal escape sequences that contaminate the input buffer.
func stripEscapeSequences(input string) string {
	var result strings.Builder
	for i := 0; i < len(input); i++ {
		if input[i] == 0x1b {
			i++ // skip ESC
			if i < len(input) && input[i] == '[' {
				// CSI sequence: skip until final byte (0x40-0x7E)
				for i++; i < len(input); i++ {
					if input[i] >= 0x40 && input[i] <= 0x7E {
						break
					}
				}
			}
			continue
		}
		result.WriteByte(input[i])
	}
	return result.String()
}

// isNetworkConfigCommand checks if the input is a network configuration command
// that may change interface state (IP assignment, link up/down, etc.)
func isNetworkConfigCommand(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return false
	}

	networkCommands := []string{
		"ip addr",
		"ip address",
		"ifconfig",
		"vtysh",
		"ip link",
	}

	for _, cmd := range networkCommands {
		if strings.HasPrefix(input, cmd) {
			return true
		}
	}

	return false
}

// parseResizeMessage attempts to parse a terminal resize control message.
// Returns cols, rows, and true if msg is a valid {"type":"resize","cols":N,"rows":N} payload.
func parseResizeMessage(msg []byte) (cols, rows uint, ok bool) {
	if len(msg) == 0 || msg[0] != '{' {
		return 0, 0, false
	}
	var ctrl struct {
		Type string `json:"type"`
		Cols uint   `json:"cols"`
		Rows uint   `json:"rows"`
	}
	if err := json.Unmarshal(msg, &ctrl); err != nil {
		return 0, 0, false
	}
	if ctrl.Type != "resize" || ctrl.Cols == 0 || ctrl.Rows == 0 {
		return 0, 0, false
	}
	return ctrl.Cols, ctrl.Rows, true
}
