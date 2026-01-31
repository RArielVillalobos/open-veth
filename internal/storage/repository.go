package storage

import "open-veth/internal/models"

// Repository define las operaciones de persistencia
type Repository interface {
	// Nodos
	SaveNode(node models.Node) error
	GetNode(id string) (models.Node, bool)
	DeleteNode(id string) error
	ListNodes() ([]models.Node, error)

	// Links
	SaveLink(link models.Link) error
	GetLink(id string) (models.Link, bool)
	DeleteLink(id string) error
	ListLinks() ([]models.Link, error)
	
	// Limpieza
	ClearAll() error
}
