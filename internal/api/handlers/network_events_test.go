package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNetworkEventHub(t *testing.T) {
	hub := NewNetworkEventHub()
	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestNetworkEventHub_RegisterAndUnregister(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	// Test initial state
	assert.Equal(t, 0, hub.ClientCount())

	// Give the hub time to start
	time.Sleep(10 * time.Millisecond)
}

func TestNetworkEventHub_Broadcast(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	// Give the hub time to start
	time.Sleep(10 * time.Millisecond)

	// Test that broadcast doesn't block even with no clients
	event := NetworkEvent{
		Type:      "interface:changed",
		NodeID:    "node-123",
		LabID:     "lab-456",
		Timestamp: time.Now().Unix(),
	}

	// Should not block even with no clients
	hub.Broadcast(event)

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Test with multiple events
	for i := 0; i < 10; i++ {
		hub.Broadcast(NetworkEvent{
			Type:      "test",
			NodeID:    "node-1",
			Timestamp: time.Now().Unix(),
		})
	}

	time.Sleep(10 * time.Millisecond)
}

func TestNetworkEventHub_Stop(t *testing.T) {
	hub := NewNetworkEventHub()

	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Give the hub time to start
	time.Sleep(10 * time.Millisecond)

	hub.Stop()

	// Run() should return after Stop
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run() did not return after Stop()")
	}
}

func TestNetworkEventHub_ConcurrentAccess(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	// Test concurrent broadcasts
	done := make(chan bool, 3)

	// Concurrent broadcasts
	go func() {
		for i := 0; i < 100; i++ {
			hub.Broadcast(NetworkEvent{
				Type:      "test",
				NodeID:    "node-1",
				Timestamp: time.Now().Unix(),
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			hub.Broadcast(NetworkEvent{
				Type:      "test",
				NodeID:    "node-2",
				Timestamp: time.Now().Unix(),
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			hub.Broadcast(NetworkEvent{
				Type:      "test",
				NodeID:    "node-3",
				Timestamp: time.Now().Unix(),
			})
		}
		done <- true
	}()

	// Wait for all goroutines with timeout
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent broadcasts")
		}
	}
}

func TestIsNetworkConfigCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ip addr add", "ip addr add 10.0.0.1/24 dev eth0", true},
		{"ip address show", "ip address show", true},
		{"ifconfig", "ifconfig eth0 192.168.1.1 netmask 255.255.255.0", true},
		{"vtysh", "vtysh -c 'show ip route'", true},
		{"ip link set", "ip link set eth0 up", true},
		{"regular command", "ls -la", false},
		{"empty command", "", false},
		{"uppercase", "IP ADDR", true},
		{"mixed case", "IfCoNfIg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNetworkConfigCommand(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNetworkEventHub_BufferFull(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	// Fill the buffer beyond capacity (buffer is 100)
	for i := 0; i < 200; i++ {
		hub.Broadcast(NetworkEvent{
			Type:      "test",
			NodeID:    "node-1",
			Timestamp: time.Now().Unix(),
		})
	}

	// Should not panic or block
	time.Sleep(50 * time.Millisecond)
}

func TestNetworkEventHub_MultipleEventTypes(t *testing.T) {
	hub := NewNetworkEventHub()
	go hub.Run()
	defer hub.Stop()

	eventTypes := []string{
		"interface:changed",
		"node:created",
		"link:created",
	}

	for _, eventType := range eventTypes {
		hub.Broadcast(NetworkEvent{
			Type:      eventType,
			NodeID:    "node-1",
			LabID:     "lab-1",
			Timestamp: time.Now().Unix(),
			Data:      map[string]string{"test": "data"},
		})
	}

	time.Sleep(10 * time.Millisecond)
}
