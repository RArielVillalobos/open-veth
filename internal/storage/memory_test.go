package storage

import (
	"fmt"
	"open-veth/internal/models"
	"sync"
	"testing"
)

func newTestRepo() *MemoryRepository {
	return NewMemoryRepository()
}

// --- Nodes ---

func TestNodeCRUD(t *testing.T) {
	repo := newTestRepo()

	node := models.Node{ID: "n1", LabID: "lab-1", Name: "HOST-1", Type: models.HOST}

	// Save
	if err := repo.SaveNode(node); err != nil {
		t.Fatalf("SaveNode failed: %v", err)
	}

	// Get
	got, ok := repo.GetNode("n1")
	if !ok {
		t.Fatal("GetNode returned not found")
	}
	if got.Name != "HOST-1" {
		t.Errorf("expected Name=HOST-1, got %q", got.Name)
	}

	// Update
	node.Name = "HOST-1-UPDATED"
	repo.SaveNode(node)
	got, _ = repo.GetNode("n1")
	if got.Name != "HOST-1-UPDATED" {
		t.Errorf("expected updated name, got %q", got.Name)
	}

	// Delete
	if err := repo.DeleteNode("n1"); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	_, ok = repo.GetNode("n1")
	if ok {
		t.Error("expected node to be deleted")
	}

	// Delete non-existent
	if err := repo.DeleteNode("n1"); err == nil {
		t.Error("expected error deleting non-existent node")
	}
}

func TestListNodes(t *testing.T) {
	repo := newTestRepo()

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1", Name: "A"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1", Name: "B"})
	repo.SaveNode(models.Node{ID: "n3", LabID: "lab-2", Name: "C"})

	// ListNodes returns all
	all, err := repo.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(all))
	}

	// ListNodesByLab filters
	lab1, err := repo.ListNodesByLab("lab-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(lab1) != 2 {
		t.Errorf("expected 2 nodes in lab-1, got %d", len(lab1))
	}

	lab2, err := repo.ListNodesByLab("lab-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(lab2) != 1 {
		t.Errorf("expected 1 node in lab-2, got %d", len(lab2))
	}

	// Empty lab
	empty, err := repo.ListNodesByLab("lab-999")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 nodes in non-existent lab, got %d", len(empty))
	}
}

// --- Links ---

func TestLinkCRUD(t *testing.T) {
	repo := newTestRepo()

	link := models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2", SourceInt: "eth0", TargetInt: "eth0"}

	if err := repo.SaveLink(link); err != nil {
		t.Fatalf("SaveLink failed: %v", err)
	}

	got, ok := repo.GetLink("l1")
	if !ok {
		t.Fatal("GetLink returned not found")
	}
	if got.SourceID != "n1" || got.TargetID != "n2" {
		t.Errorf("unexpected link data: %+v", got)
	}

	if err := repo.DeleteLink("l1"); err != nil {
		t.Fatalf("DeleteLink failed: %v", err)
	}
	_, ok = repo.GetLink("l1")
	if ok {
		t.Error("expected link to be deleted")
	}

	if err := repo.DeleteLink("l1"); err == nil {
		t.Error("expected error deleting non-existent link")
	}
}

func TestListLinksByLab(t *testing.T) {
	repo := newTestRepo()

	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l3", LabID: "lab-2"})

	lab1, _ := repo.ListLinksByLab("lab-1")
	if len(lab1) != 2 {
		t.Errorf("expected 2 links in lab-1, got %d", len(lab1))
	}
}

// --- Laboratories ---

func TestLaboratoryCRUD(t *testing.T) {
	repo := newTestRepo()

	lab := models.Laboratory{ID: "lab-1", Name: "Test Lab"}

	if err := repo.SaveLaboratory(lab); err != nil {
		t.Fatalf("SaveLaboratory failed: %v", err)
	}

	got, ok := repo.GetLaboratory("lab-1")
	if !ok {
		t.Fatal("GetLaboratory returned not found")
	}
	if got.Name != "Test Lab" {
		t.Errorf("expected name 'Test Lab', got %q", got.Name)
	}

	labs, _ := repo.ListLaboratories()
	if len(labs) != 1 {
		t.Errorf("expected 1 lab, got %d", len(labs))
	}
}

func TestDeleteLaboratoryCascade(t *testing.T) {
	repo := newTestRepo()

	// Setup: lab with nodes, links, and interface configs
	repo.SaveLaboratory(models.Laboratory{ID: "lab-1", Name: "Lab"})
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1"})
	repo.SaveNode(models.Node{ID: "n3", LabID: "lab-2"}) // different lab
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-2"}) // different lab
	repo.SaveInterfaceConfigs("lab-1", []models.InterfaceConfig{
		{NodeID: "n1", Interface: "eth0", Address: "10.0.0.1/24"},
	})
	repo.SaveInterfaceConfigs("lab-2", []models.InterfaceConfig{
		{NodeID: "n3", Interface: "eth0", Address: "10.0.1.1/24"},
	})

	if err := repo.DeleteLaboratory("lab-1"); err != nil {
		t.Fatalf("DeleteLaboratory failed: %v", err)
	}

	// Lab deleted
	_, ok := repo.GetLaboratory("lab-1")
	if ok {
		t.Error("expected lab to be deleted")
	}

	// Nodes from lab-1 deleted
	_, ok = repo.GetNode("n1")
	if ok {
		t.Error("expected n1 to be cascade deleted")
	}
	_, ok = repo.GetNode("n2")
	if ok {
		t.Error("expected n2 to be cascade deleted")
	}

	// Node from lab-2 survives
	_, ok = repo.GetNode("n3")
	if !ok {
		t.Error("expected n3 (lab-2) to survive")
	}

	// Link from lab-1 deleted
	_, ok = repo.GetLink("l1")
	if ok {
		t.Error("expected l1 to be cascade deleted")
	}

	// Link from lab-2 survives
	_, ok = repo.GetLink("l2")
	if !ok {
		t.Error("expected l2 (lab-2) to survive")
	}

	// Interface configs from lab-1 deleted
	configs, _ := repo.GetInterfaceConfigsByLab("lab-1")
	if len(configs) != 0 {
		t.Error("expected lab-1 interface configs to be cascade deleted")
	}

	// Interface configs from lab-2 survive
	configs2, _ := repo.GetInterfaceConfigsByLab("lab-2")
	if len(configs2) != 1 {
		t.Error("expected lab-2 interface configs to survive")
	}
}

func TestDeleteNodeCascade(t *testing.T) {
	repo := newTestRepo()

	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1"})
	repo.SaveNode(models.Node{ID: "n2", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1", SourceID: "n2", TargetID: "n1"})
	repo.SaveLink(models.Link{ID: "l3", LabID: "lab-1", SourceID: "n2", TargetID: "n2"}) // unrelated
	repo.SaveInterfaceConfigs("lab-1", []models.InterfaceConfig{
		{NodeID: "n1", Interface: "eth0", Address: "10.0.0.1/24"},
		{NodeID: "n2", Interface: "eth0", Address: "10.0.0.2/24"},
	})

	if err := repo.DeleteNode("n1"); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// n1 gone
	_, ok := repo.GetNode("n1")
	if ok {
		t.Error("expected n1 to be deleted")
	}

	// Links referencing n1 gone
	_, ok = repo.GetLink("l1")
	if ok {
		t.Error("expected l1 to be cascade deleted")
	}
	_, ok = repo.GetLink("l2")
	if ok {
		t.Error("expected l2 to be cascade deleted")
	}

	// Unrelated link survives
	_, ok = repo.GetLink("l3")
	if !ok {
		t.Error("expected l3 to survive")
	}

	// Interface configs for n1 gone, n2 survives
	configs, _ := repo.GetInterfaceConfigsByLab("lab-1")
	if len(configs) != 1 {
		t.Errorf("expected 1 config (n2's), got %d", len(configs))
	}
	if len(configs) > 0 && configs[0].NodeID != "n2" {
		t.Errorf("expected surviving config to belong to n2, got %s", configs[0].NodeID)
	}

	// n2 still exists
	_, ok = repo.GetNode("n2")
	if !ok {
		t.Error("expected n2 to survive")
	}
}

func TestListLinksByNode(t *testing.T) {
	repo := newTestRepo()

	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1", SourceID: "n1", TargetID: "n2"})
	repo.SaveLink(models.Link{ID: "l2", LabID: "lab-1", SourceID: "n2", TargetID: "n3"})
	repo.SaveLink(models.Link{ID: "l3", LabID: "lab-1", SourceID: "n3", TargetID: "n1"})

	// n1 is source in l1, target in l3
	links, err := repo.ListLinksByNode("n1")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links for n1, got %d", len(links))
	}

	// n2 is target in l1, source in l2
	links, _ = repo.ListLinksByNode("n2")
	if len(links) != 2 {
		t.Errorf("expected 2 links for n2, got %d", len(links))
	}

	// Non-existent node
	links, _ = repo.ListLinksByNode("n999")
	if len(links) != 0 {
		t.Errorf("expected 0 links for non-existent node, got %d", len(links))
	}
}

// --- Interface Configs ---

func TestInterfaceConfigs(t *testing.T) {
	repo := newTestRepo()

	configs := []models.InterfaceConfig{
		{LabID: "lab-1", NodeID: "n1", Interface: "eth0", Address: "10.0.0.1/24"},
		{LabID: "lab-1", NodeID: "n1", Interface: "eth1", Address: "10.0.1.1/24"},
	}

	if err := repo.SaveInterfaceConfigs("lab-1", configs); err != nil {
		t.Fatalf("SaveInterfaceConfigs failed: %v", err)
	}

	got, err := repo.GetInterfaceConfigsByLab("lab-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 configs, got %d", len(got))
	}

	// Overwrite replaces
	repo.SaveInterfaceConfigs("lab-1", []models.InterfaceConfig{
		{LabID: "lab-1", NodeID: "n1", Interface: "eth0", Address: "192.168.1.1/24"},
	})
	got, _ = repo.GetInterfaceConfigsByLab("lab-1")
	if len(got) != 1 {
		t.Errorf("expected 1 config after overwrite, got %d", len(got))
	}

	// Empty lab returns nil/empty
	empty, _ := repo.GetInterfaceConfigsByLab("lab-999")
	if len(empty) != 0 {
		t.Errorf("expected 0 configs for non-existent lab, got %d", len(empty))
	}
}

// --- ClearAll ---

func TestClearAll(t *testing.T) {
	repo := newTestRepo()

	repo.SaveLaboratory(models.Laboratory{ID: "lab-1"})
	repo.SaveNode(models.Node{ID: "n1", LabID: "lab-1"})
	repo.SaveLink(models.Link{ID: "l1", LabID: "lab-1"})
	repo.SaveInterfaceConfigs("lab-1", []models.InterfaceConfig{{NodeID: "n1"}})

	if err := repo.ClearAll(); err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	nodes, _ := repo.ListNodes()
	links, _ := repo.ListLinks()
	labs, _ := repo.ListLaboratories()
	configs, _ := repo.GetInterfaceConfigsByLab("lab-1")

	if len(nodes) != 0 || len(links) != 0 || len(labs) != 0 || len(configs) != 0 {
		t.Errorf("expected all empty after ClearAll, got nodes=%d links=%d labs=%d configs=%d",
			len(nodes), len(links), len(labs), len(configs))
	}
}

// --- Concurrency ---

// TestConcurrentNodeAccess verifies the MemoryRepository doesn't race
// under concurrent reads and writes. Run with -race to detect data races.
func TestConcurrentNodeAccess(t *testing.T) {
	repo := newTestRepo()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // writers + readers + deleters

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			node := models.Node{
				ID:    fmt.Sprintf("n%d", id),
				LabID: "lab-1",
				Name:  fmt.Sprintf("HOST-%d", id),
			}
			repo.SaveNode(node)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			repo.GetNode(fmt.Sprintf("n%d", id))
			repo.ListNodes()
			repo.ListNodesByLab("lab-1")
		}(i)
	}

	// Concurrent deleters (some will fail, that's fine)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_ = repo.DeleteNode(fmt.Sprintf("n%d", id))
		}(i)
	}

	wg.Wait()
}

func TestConcurrentMixedOperations(t *testing.T) {
	repo := newTestRepo()
	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	// Save nodes
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			repo.SaveNode(models.Node{ID: fmt.Sprintf("n%d", id), LabID: "lab-1"})
		}(i)
	}

	// Save links
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			repo.SaveLink(models.Link{ID: fmt.Sprintf("l%d", id), LabID: "lab-1"})
		}(i)
	}

	// Save labs
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			repo.SaveLaboratory(models.Laboratory{ID: fmt.Sprintf("lab-%d", id)})
		}(i)
	}

	// ClearAll racing with everything
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			repo.ClearAll()
		}()
	}

	wg.Wait()

	// No assertion on final state — the point is no panics or races
}
