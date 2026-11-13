package handlers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"open-veth/internal/models"
	"open-veth/internal/storage"
)

type stateReaderStub struct {
	interfaces      map[string][]models.InterfaceInfo
	routes          map[string][]models.RouteInfo
	interfaceErrors map[string]error
	routeErrors     map[string]error
}

func (s stateReaderStub) GetNodeInterfaces(_ context.Context, id string) ([]models.InterfaceInfo, error) {
	return s.interfaces[id], s.interfaceErrors[id]
}

func (s stateReaderStub) GetNodeRoutes(_ context.Context, id string) ([]models.RouteInfo, error) {
	return s.routes[id], s.routeErrors[id]
}

func TestCaptureLabStatePreservesUnreadableNodes(t *testing.T) {
	for _, stage := range []string{"interfaces", "routes", "stopped", "missing-runtime", "cancelled", "empty"} {
		t.Run(stage, func(t *testing.T) {
			repo := storage.NewMemoryRepository()
			oldIPs := []models.InterfaceConfig{{LabID: "lab", NodeID: "n1", Interface: "eth0", Address: "10.0.0.1/24"}}
			oldRoutes := []models.RouteConfig{{LabID: "lab", NodeID: "n1", Dst: "default", Gateway: "10.0.0.254", Dev: "eth0"}}
			if err := repo.SaveLabConfigs("lab", oldIPs, oldRoutes); err != nil {
				t.Fatal(err)
			}
			nodes := []models.Node{{ID: "n1", ContainerID: "c1", Status: models.NodeStatusRunning}, {ID: "n2", ContainerID: "c2", Status: models.NodeStatusRunning}}
			reader := stateReaderStub{interfaceErrors: map[string]error{}, routeErrors: map[string]error{}}
			switch stage {
			case "interfaces":
				reader.interfaceErrors["c1"] = errors.New("Docker unavailable")
			case "routes":
				reader.routeErrors["c1"] = errors.New("exec failed")
			case "cancelled":
				reader.routeErrors["c1"] = context.DeadlineExceeded
			case "stopped":
				nodes[0].Status = models.NodeStatusStopped
			case "missing-runtime":
				nodes[0].ContainerID = ""
			}
			ips, routes, captured, err := captureLabState(context.Background(), reader, repo, "lab", nodes)
			wantError := stage == "interfaces" || stage == "routes" || stage == "cancelled"
			if (err != nil) != wantError {
				t.Fatalf("unexpected capture error: %v", err)
			}
			if !captured {
				t.Fatal("healthy second node must still be captured")
			}
			if stage == "empty" {
				if len(ips) != 0 || len(routes) != 0 {
					t.Fatal("a successful empty read must clear previous configuration")
				}
			} else if !reflect.DeepEqual(ips, oldIPs) || !reflect.DeepEqual(routes, oldRoutes) {
				t.Fatalf("lost previous configuration: IPs=%v routes=%v", ips, routes)
			}
			if err := repo.SaveLabConfigs("lab", ips, routes); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCaptureLabStateAllReadsFail(t *testing.T) {
	repo := storage.NewMemoryRepository()
	reader := stateReaderStub{interfaceErrors: map[string]error{"c1": context.DeadlineExceeded}}
	_, _, captured, err := captureLabState(context.Background(), reader, repo, "lab", []models.Node{{ID: "n1", ContainerID: "c1", Status: models.NodeStatusRunning}})
	if captured || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("captured=%v err=%v", captured, err)
	}
}
