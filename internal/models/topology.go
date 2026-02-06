package models

import "fmt"

// NodeType define el tipo de dispositivo (router, switch, host)
type NodeType string

const (
	ROUTER NodeType = "router" // Usa imagen FRR/Quagga
	SWITCH NodeType = "switch" // Usa Linux Bridge (nativo)
	HOST   NodeType = "host"   // Usa imagen Alpine/Ubuntu
)

// Official Images
const (
	ImgRouter = "openveth/router:latest"
	ImgHost   = "openveth/host:latest"
)

// GetImageForType returns the official docker image for a given node type
func GetImageForType(t NodeType) string {
	switch t {
	case ROUTER:
		return ImgRouter
	case HOST:
		return ImgHost
	case SWITCH:
		return ImgHost // Switch uses the host image to run the bridge
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

	// Internal state
	ContainerID string `json:"container_id"`
	PID         int    `json:"pid"`

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
	SourceID  string `json:"source"`
	TargetID  string `json:"target"`
	SourceInt string `json:"source_int"`
	TargetInt string `json:"target_int"`
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

// Topology is the object that encompasses a full laboratory state for frontend
type Topology struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// --- Export Models (Clean YAML) ---

type LabExport struct {
	Version string       `yaml:"version"`
	Name    string       `yaml:"name"`
	Nodes   []NodeExport `yaml:"nodes"`
	Links   []LinkExport `yaml:"links"`
}

type NodeExport struct {
	ID   string   `yaml:"id"`
	Name string   `yaml:"name"`
	Type NodeType `yaml:"type"`
	// Image removed for security abstraction
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
}

type LinkExport struct {
	ID        string `yaml:"id"`
	Source    string `yaml:"source"`
	Target    string `yaml:"target"`
	SourceInt string `yaml:"source_int"`
	TargetInt string `yaml:"target_int"`
}

// FormatMAC converts a hardware address byte slice to "aa:bb:cc:dd:ee:ff"
func FormatMAC(mac []byte) string {
	if len(mac) < 6 {
		return "00:00:00:00:00:00"
	}
	return string(fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]))
}
