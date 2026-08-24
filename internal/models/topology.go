package models

import "fmt"

// NodeType define el tipo de dispositivo (router, switch, host)
type NodeType string

const (
	ROUTER  NodeType = "router"  // Uses FRR/Quagga image
	SWITCH  NodeType = "switch"  // Uses Linux Bridge (native)
	HUB     NodeType = "hub"     // Uses Linux Bridge without MAC learning (L1 repeater)
	HOST    NodeType = "host"    // Uses Alpine/Ubuntu image
	CLOUD   NodeType = "cloud"   // Internet gateway - keeps eth0 connected to Docker bridge
	SERVER  NodeType = "server"  // Debian with systemd — for sysadmin, services and automation labs
	MONITOR NodeType = "monitor" // Grafana + Prometheus pre-configured — for observability labs
	TESTER  NodeType = "tester"  // Debian with wrk, k6, siege, iperf3, locust — for load and stress testing
	HAPROXY NodeType = "haproxy" // Alpine + HAProxy — for load balancing and reverse proxy labs
	STORAGE NodeType = "storage" // Debian with systemd + privileged — loop devices, LVM, RAID, LUKS, NFS
)

// Node status constants
const (
	NodeStatusRunning = "running"
	NodeStatusStopped = "stopped"
)

// NeedsBridge returns true for node types that use an internal bridge (br0)
func NeedsBridge(t NodeType) bool {
	return t == SWITCH || t == HUB
}

// IsValidNodeType returns true if the given type is a known node type
func IsValidNodeType(t NodeType) bool {
	switch t {
	case ROUTER, SWITCH, HUB, HOST, CLOUD, SERVER, MONITOR, TESTER, HAPROXY, STORAGE:
		return true
	}
	return false
}


// SnapshotImageName returns the local Docker image name used to snapshot a node's filesystem.
func SnapshotImageName(nodeID string) string {
	return "openveth-snapshot:" + nodeID
}

// Official Images
const (
	ImgRouter  = "openveth/router:latest"
	ImgHost    = "openveth/host:latest"
	ImgServer  = "openveth/server:latest"
	ImgMonitor = "openveth/monitor:latest"
	ImgTester  = "openveth/tester:latest"
	ImgSwitch  = "openveth/switch:latest"
	ImgHAProxy = "openveth/haproxy:latest"
	ImgStorage = "openveth/storage:latest"
)

// GetImageForType returns the official docker image for a given node type
func GetImageForType(t NodeType) string {
	switch t {
	case ROUTER:
		return ImgRouter
	case HOST:
		return ImgHost
	case SWITCH:
		return ImgSwitch // Managed switch based on Debian + SNMP
	case HUB:
		return ImgHost // Hub remains a simple, unmanaged device using Alpine
	case CLOUD:
		return ImgHost // Cloud uses host image, keeps eth0 for internet access
	case SERVER:
		return ImgServer
	case MONITOR:
		return ImgMonitor
	case TESTER:
		return ImgTester
	case HAPROXY:
		return ImgHAProxy
	case STORAGE:
		return ImgStorage
	default:
		return ImgHost
	}
}

// Node represents a device in the network
type Node struct {
	ID         string   `json:"id" gorm:"primaryKey"`
	LabID      string   `json:"lab_id" gorm:"index"` // Associated laboratory
	Name       string   `json:"name"`
	Type       NodeType `json:"type"`
	Image      string   `json:"image"`
	CPURequest string   `json:"cpu_request"`
	RAMLimit   string   `json:"ram_limit"`
	X          float64  `json:"x"` // Canvas position
	Y          float64  `json:"y"` // Canvas position
	Status     string   `json:"status" gorm:"default:running"` // NodeStatusRunning | NodeStatusStopped

	// Snapshot: local image created by "Save State" — used on next lab activation instead of base image
	SnapshotImage string `json:"snapshot_image,omitempty"`

	// Runtime state (not persisted, rebuilt from Docker on startup)
	ContainerID  string         `json:"container_id" gorm:"-"`
	PID          int            `json:"pid" gorm:"-"`
	ServicePorts map[string]int `json:"service_ports,omitempty" gorm:"-"`

	// Runtime Info (Not persisted in DB)
	Interfaces []InterfaceInfo `json:"interfaces" gorm:"-"`
}

// InterfaceInfo maps the output of 'ip -j addr'
type InterfaceInfo struct {
	Name        string      `json:"ifname"`
	IPAddresses []IPAddress `json:"addr_info"`
}

type IPAddress struct {
	Address string `json:"local"`
	Prefix  int    `json:"prefixlen"`
}

// MacEntry represents a single entry in a bridge MAC address table (bridge fdb show)
type MacEntry struct {
	MAC    string `json:"mac"`
	Port   string `json:"port"`
	Type   string `json:"type"` // "static" | "dynamic"
}

// RouteInfo maps the output of 'ip -j route'
type RouteInfo struct {
	Dst      string `json:"dst"`
	Gateway  string `json:"gateway,omitempty"`
	Dev      string `json:"dev"`
	Protocol string `json:"protocol"`
	Scope    string `json:"scope"`
	PrefSrc  string `json:"prefsrc,omitempty"`
	Metric   int    `json:"metric,omitempty"`
}

// Link represents a virtual cable (veth pair) between two nodes
type Link struct {
	ID        string `json:"id" gorm:"primaryKey"`
	LabID     string `json:"lab_id" gorm:"index"` // Associated laboratory
	SourceID  string `json:"source" gorm:"index"`
	TargetID  string `json:"target" gorm:"index"`
	SourceInt string `json:"source_int"`
	TargetInt string `json:"target_int"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
}

// Laboratory represents a saved network project
type Laboratory struct {
	ID        string `json:"id" gorm:"primaryKey"`
	UserID    string `json:"user_id" gorm:"index"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// InterfaceConfig stores saved IP configuration for node interfaces
type InterfaceConfig struct {
	ID        uint   `json:"-" gorm:"primaryKey;autoIncrement"`
	LabID     string `json:"lab_id" gorm:"index"`
	NodeID    string `json:"node_id" gorm:"index"`
	Interface string `json:"interface"`
	Address   string `json:"address"` // CIDR format: "192.168.1.1/24"
}

// RouteConfig stores saved static route configuration for node interfaces
type RouteConfig struct {
	ID      uint   `json:"-" gorm:"primaryKey;autoIncrement"`
	LabID   string `json:"lab_id" gorm:"index"`
	NodeID  string `json:"node_id" gorm:"index"`
	Dst     string `json:"dst"`     // e.g., "10.0.2.0/24"
	Gateway string `json:"gateway"` // e.g., "10.0.1.1"
	Dev     string `json:"dev"`     // e.g., "eth0"
}

// Topology is the object that encompasses a full laboratory state for frontend
type Topology struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// --- Export Models (Clean YAML) ---

type LabExport struct {
	Name  string       `yaml:"name"`
	Nodes []NodeExport `yaml:"nodes"`
	Links []LinkExport `yaml:"links"`
}

type NodeExport struct {
	Name string   `yaml:"name"`
	Type NodeType `yaml:"type"`
	// Image removed for security abstraction
	X          float64           `yaml:"x"`
	Y          float64           `yaml:"y"`
	Interfaces []InterfaceExport `yaml:"interfaces,omitempty"`
	Routes     []RouteExport     `yaml:"routes,omitempty"`
}

type InterfaceExport struct {
	Interface string `yaml:"interface"`
	Address   string `yaml:"address"`
}

type RouteExport struct {
	Dst     string `yaml:"dst"`
	Gateway string `yaml:"gateway,omitempty"`
	Dev     string `yaml:"dev"`
}

type LinkExport struct {
	Source    string `yaml:"source"`     // node name
	Target    string `yaml:"target"`     // node name
	SourceInt string `yaml:"source_int"`
	TargetInt string `yaml:"target_int"`
	Enabled   bool   `yaml:"enabled"`
}

// FormatMAC converts a hardware address byte slice to "aa:bb:cc:dd:ee:ff"
func FormatMAC(mac []byte) string {
	if len(mac) < 6 {
		return "00:00:00:00:00:00"
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}
