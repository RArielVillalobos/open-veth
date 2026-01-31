package storage

import (
	"fmt"
	"open-veth/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository initializes a database connection based on the driver
func NewGormRepository(driver string, dsn string) (*GormRepository, error) {
	var dialector gorm.Dialector

	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database (%s): %v", driver, err)
	}

	// Auto Migrate models
	err = db.AutoMigrate(&models.Node{}, &models.Link{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	return &GormRepository{db: db}, nil
}

func (r *GormRepository) SaveNode(node models.Node) error {
	return r.db.Save(&node).Error
}

func (r *GormRepository) GetNode(id string) (models.Node, bool) {
	var node models.Node
	if err := r.db.First(&node, "id = ?", id).Error; err != nil {
		return models.Node{}, false
	}
	return node, true
}

func (r *GormRepository) DeleteNode(id string) error {
	return r.db.Delete(&models.Node{}, "id = ?", id).Error
}

func (r *GormRepository) ListNodes() ([]models.Node, error) {
	var nodes []models.Node
	err := r.db.Find(&nodes).Error
	return nodes, err
}

func (r *GormRepository) SaveLink(link models.Link) error {
	return r.db.Save(&link).Error
}

func (r *GormRepository) GetLink(id string) (models.Link, bool) {
	var link models.Link
	if err := r.db.First(&link, "id = ?", id).Error; err != nil {
		return models.Link{}, false
	}
	return link, true
}

func (r *GormRepository) DeleteLink(id string) error {
	return r.db.Delete(&models.Link{}, "id = ?", id).Error
}

func (r *GormRepository) ListLinks() ([]models.Link, error) {
	var links []models.Link
	err := r.db.Find(&links).Error
	return links, err
}

func (r *GormRepository) ClearAll() error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM nodes").Error; err != nil { return err }
		if err := tx.Exec("DELETE FROM links").Error; err != nil { return err }
		return nil
	})
}
