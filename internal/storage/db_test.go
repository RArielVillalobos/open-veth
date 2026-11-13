package storage

import (
	"fmt"
	"strings"
	"testing"

	"open-veth/internal/models"

	"gorm.io/gorm"
)

func TestSaveLabConfigsRollback(t *testing.T) {
	repo, err := NewGormRepository("sqlite", ":memory:")
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("SQLite requires CGO")
		}
		t.Fatal(err)
	}
	oldIPs := []models.InterfaceConfig{{NodeID: "n1", Address: "10.0.0.1/24"}}
	oldRoutes := []models.RouteConfig{{NodeID: "n1", Dst: "default", Gateway: "10.0.0.254"}}
	if err := repo.SaveLabConfigs("lab", oldIPs, oldRoutes); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Callback().Create().Before("gorm:create").Register("fail_routes", func(tx *gorm.DB) {
		if tx.Statement.Table == "route_configs" {
			tx.AddError(fmt.Errorf("simulated route write failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	err = repo.SaveLabConfigs("lab", []models.InterfaceConfig{{NodeID: "n1", Address: "10.0.0.2/24"}}, oldRoutes)
	if err == nil {
		t.Fatal("expected route write failure")
	}
	ips, err := repo.GetInterfaceConfigsByLab("lab")
	if err != nil {
		t.Fatal(err)
	}
	routes, err := repo.GetRouteConfigsByLab("lab")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || ips[0].Address != "10.0.0.1/24" || len(routes) != 1 || routes[0].Gateway != "10.0.0.254" {
		t.Fatalf("transaction did not preserve previous state: %v %v", ips, routes)
	}
}

func TestSaveLinkPreservesDisabledState(t *testing.T) {
	repo, err := NewGormRepository("sqlite", ":memory:")
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("SQLite requires CGO")
		}
		t.Fatal(err)
	}
	if err := repo.SaveLink(models.Link{ID: "disabled", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	link, found := repo.GetLink("disabled")
	if !found || link.Enabled {
		t.Fatal("disabled link was persisted as enabled")
	}
}
