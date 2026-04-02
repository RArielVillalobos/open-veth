package orchestrator

import (
	"testing"
)

func TestParseFdbOutput_DynamicEntry(t *testing.T) {
	output := `0e:72:9a:9f:82:13 dev eth1 master br0`

	entries := parseFdbOutput(output)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MAC != "0e:72:9a:9f:82:13" {
		t.Errorf("MAC = %q, want %q", entries[0].MAC, "0e:72:9a:9f:82:13")
	}
	if entries[0].Port != "eth1" {
		t.Errorf("Port = %q, want %q", entries[0].Port, "eth1")
	}
	if entries[0].Type != "dynamic" {
		t.Errorf("Type = %q, want %q", entries[0].Type, "dynamic")
	}
}

func TestParseFdbOutput_StaticEntry(t *testing.T) {
	output := `aa:bb:cc:dd:ee:ff dev eth2 master br0 static`

	entries := parseFdbOutput(output)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Type != "static" {
		t.Errorf("Type = %q, want %q", entries[0].Type, "static")
	}
}

func TestParseFdbOutput_SkipsPermanentEntries(t *testing.T) {
	// Permanent entries are the port's own MAC registered by the bridge kernel
	output := `02:83:67:ad:bd:e7 dev eth1 master br0 permanent`

	entries := parseFdbOutput(output)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries (permanent skipped), got %d", len(entries))
	}
}

func TestParseFdbOutput_SkipsSelfEntries(t *testing.T) {
	// Self entries are per-interface infrastructure entries
	output := `02:83:67:ad:bd:e7 dev eth1 self permanent`

	entries := parseFdbOutput(output)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries (self skipped), got %d", len(entries))
	}
}

func TestParseFdbOutput_SkipsMulticastIPv6(t *testing.T) {
	output := `33:33:00:00:00:01 dev eth1 self permanent
33:33:ff:ad:bd:e7 dev eth1 self permanent`

	entries := parseFdbOutput(output)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries (multicast IPv6 skipped), got %d", len(entries))
	}
}

func TestParseFdbOutput_SkipsMulticastIPv4(t *testing.T) {
	output := `01:00:5e:00:00:01 dev eth1 self permanent`

	entries := parseFdbOutput(output)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries (multicast IPv4 skipped), got %d", len(entries))
	}
}

func TestParseFdbOutput_SkipsBr0Port(t *testing.T) {
	// Entries on br0 itself should be filtered
	output := `aa:bb:cc:dd:ee:ff dev br0 master br0`

	entries := parseFdbOutput(output)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries (br0 port skipped), got %d", len(entries))
	}
}

func TestParseFdbOutput_RealWorldOutput(t *testing.T) {
	// Realistic output from a switch with one connected server (eth1)
	output := `0e:72:9a:9f:82:13 dev eth1 master br0
02:83:67:ad:bd:e7 dev eth1 master br0 permanent
02:83:67:ad:bd:e7 dev eth1 self permanent
33:33:00:00:00:01 dev eth1 self permanent
33:33:ff:ad:bd:e7 dev eth1 self permanent`

	entries := parseFdbOutput(output)

	// Only the dynamic unicast entry should survive
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].MAC != "0e:72:9a:9f:82:13" {
		t.Errorf("MAC = %q, want %q", entries[0].MAC, "0e:72:9a:9f:82:13")
	}
	if entries[0].Port != "eth1" {
		t.Errorf("Port = %q, want %q", entries[0].Port, "eth1")
	}
	if entries[0].Type != "dynamic" {
		t.Errorf("Type = %q, want %q", entries[0].Type, "dynamic")
	}
}

func TestParseFdbOutput_MultiplePortsMultipleMACs(t *testing.T) {
	output := `0e:72:9a:9f:82:13 dev eth1 master br0
aa:bb:cc:dd:ee:01 dev eth2 master br0
aa:bb:cc:dd:ee:02 dev eth2 master br0`

	entries := parseFdbOutput(output)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[1].Port != "eth2" || entries[2].Port != "eth2" {
		t.Errorf("expected eth2 for entries 1 and 2, got %q, %q", entries[1].Port, entries[2].Port)
	}
}

func TestParseFdbOutput_Empty(t *testing.T) {
	entries := parseFdbOutput("")

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty output, got %d", len(entries))
	}
}
