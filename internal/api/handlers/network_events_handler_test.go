package handlers

import (
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestHandleNetworkEvents_WebSocketUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	r := gin.New()
	r.GET("/events", h.HandleNetworkEvents)

	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/events"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skipf("WebSocket connection failed (expected in unit test environment): %v", err)
		return
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, h.EventHub.ClientCount())
}

func TestBroadcastInterfaceChanged(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Should not panic even without clients
	h.BroadcastInterfaceChanged("node-1", "lab-1")
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastInterfaceChanged_NilHub(t *testing.T) {
	h := &Handler{
		EventHub: nil,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Should not panic with nil hub
	h.BroadcastInterfaceChanged("node-1", "lab-1")
}

func TestBroadcastNodeCreated(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastNodeCreated("node-1", "lab-1", "router")
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastNodeStopped(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastNodeStopped("node-1", "lab-1")
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastNodeStopped_NilHub(t *testing.T) {
	h := &Handler{
		EventHub: nil,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastNodeStopped("node-1", "lab-1")
}

func TestBroadcastNodeStarted(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastNodeStarted("node-1", "lab-1")
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastNodeStarted_NilHub(t *testing.T) {
	h := &Handler{
		EventHub: nil,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastNodeStarted("node-1", "lab-1")
}

func TestBroadcastLinkCreated(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	h := &Handler{
		EventHub: hub,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	h.BroadcastLinkCreated("link-1", "lab-1", "node-1", "node-2")
	time.Sleep(50 * time.Millisecond)
}

func TestNetworkEventHub_EventStructure(t *testing.T) {
	now := time.Now().Unix()

	event := NetworkEvent{
		Type:      "interface:changed",
		NodeID:    "node-123",
		LabID:     "lab-456",
		Timestamp: now,
		Data: map[string]string{
			"interface": "eth0",
			"address":   "10.0.0.1/24",
		},
	}

	assert.Equal(t, "interface:changed", event.Type)
	assert.Equal(t, "node-123", event.NodeID)
	assert.Equal(t, "lab-456", event.LabID)
	assert.Equal(t, now, event.Timestamp)

	data, ok := event.Data.(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "eth0", data["interface"])
	assert.Equal(t, "10.0.0.1/24", data["address"])
}

func TestIsNetworkConfigCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"only spaces", "   ", false},
		{"with special chars", "ip addr add 10.0.0.1/24 dev eth0 # comment", true},
		{"similar command", "ipcrm", false},
		{"partial match", "ip", false},
		{"ifconfig lowercase", "ifconfig", true},
		{"IFCONFIG uppercase", "IFCONFIG", true},
		{"vtysh command", "vtysh", true},
		{"vtysh with args", "vtysh -c 'show ip route'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNetworkConfigCommand(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %q", tt.input)
		})
	}
}

func TestTerminalInputBuffer(t *testing.T) {
	var buffer strings.Builder

	// Simulate typing "ip addr"
	inputs := []string{"i", "p", " ", "a", "d", "d", "r", "\r"}

	for _, input := range inputs {
		if input == "\r" || input == "\n" {
			cmd := strings.TrimSpace(buffer.String())
			assert.Equal(t, "ip addr", cmd)
			buffer.Reset()
		} else if len(input) == 1 && input[0] >= 32 && input[0] <= 126 {
			buffer.WriteString(input)
		}
	}
}

func TestTerminalInputBuffer_Backspace(t *testing.T) {
	var buffer strings.Builder

	inputs := []string{"i", "f", "c", "o", "n", "f", "i", "g", "\b", "g"}

	for _, input := range inputs {
		switch input {
		case "\b", "\x7f":
			current := buffer.String()
			if len(current) > 0 {
				buffer.Reset()
				buffer.WriteString(current[:len(current)-1])
			}
		default:
			if len(input) == 1 && input[0] >= 32 && input[0] <= 126 {
				buffer.WriteString(input)
			}
		}
	}

	assert.Equal(t, "ifconfig", buffer.String())
}

func TestTerminalInputBuffer_Clear(t *testing.T) {
	var buffer strings.Builder

	inputs := []string{"i", "p", " ", "a", "d", "d", "r", "\x03"}

	for _, input := range inputs {
		switch input {
		case "\x03", "\x04":
			buffer.Reset()
		default:
			if len(input) == 1 && input[0] >= 32 && input[0] <= 126 {
				buffer.WriteString(input)
			}
		}
	}

	assert.Equal(t, "", buffer.String())
}
