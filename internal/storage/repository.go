package storage

import "open-veth/internal/models"

// Repository defines the persistence operations
type Repository interface {
	// Nodes
	SaveNode(node models.Node) error
	GetNode(id string) (models.Node, bool)
	DeleteNode(id string) error
	ListNodes() ([]models.Node, error)
	ListNodesByLab(labID string) ([]models.Node, error)

	// Links
	SaveLink(link models.Link) error
	GetLink(id string) (models.Link, bool)
	DeleteLink(id string) error
	ListLinks() ([]models.Link, error)
	ListLinksByLab(labID string) ([]models.Link, error)

	// Laboratories
	SaveLaboratory(lab models.Laboratory) error
	GetLaboratory(id string) (models.Laboratory, bool)
	ListLaboratories() ([]models.Laboratory, error)
	DeleteLaboratory(id string) error

	// Cleanup
	ClearAll() error
}
