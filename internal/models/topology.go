package models

// NodeType define el tipo de dispositivo (router, switch, host)
type NodeType string

const (
	ROUTER NodeType = "router" // Usa imagen FRR/Quagga
	SWITCH NodeType = "switch" // Usa Linux Bridge (nativo)
	HOST   NodeType = "host"   // Usa imagen Alpine/Ubuntu
)

// Node representa un dispositivo en la red
type Node struct {
	ID          string   `json:"id" gorm:"primaryKey"`
	Name        string   `json:"name"`
	Type        NodeType `json:"type"`
	Image       string   `json:"image"`
	CPURequest  string   `json:"cpu_request"`
	RAMLimit    string   `json:"ram_limit"`
	X           float64  `json:"x"` // Canvas position
	Y           float64  `json:"y"` // Canvas position
	
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

// Link representa un cable virtual (veth pair) entre dos nodos
type Link struct {
	ID        string `json:"id" gorm:"primaryKey"`
	SourceID  string `json:"source"`
	TargetID  string `json:"target"`
	SourceInt string `json:"source_int"`
	TargetInt string `json:"target_int"`
}

// Topology es el objeto que engloba un laboratorio completo
type Topology struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}
