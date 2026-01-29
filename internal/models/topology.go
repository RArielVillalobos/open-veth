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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        NodeType `json:"type"`
	Image       string   `json:"image"`       // Ej: "alpine:latest" o "frrouting/frr"
	CPURequest  string   `json:"cpu_request"` // Ej: "0.1" (10% de un core)
	RAMLimit    string   `json:"ram_limit"`   // Ej: "128MB"
	
	// Estado en tiempo real (No se guarda en DB necesariamente)
	ContainerID string   `json:"-"` // ID interno de Docker
	PID         int      `json:"-"` // PID del proceso para Netns plumbing
}

// Link representa un cable virtual (veth pair) entre dos nodos
type Link struct {
	ID        string `json:"id"`
	SourceID  string `json:"source"`
	TargetID  string `json:"target"`
	SourceInt string `json:"source_int"` // Nombre interfaz (eth0)
	TargetInt string `json:"target_int"` // Nombre interfaz (eth0)
}

// Topology es el objeto que engloba un laboratorio completo
type Topology struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}
