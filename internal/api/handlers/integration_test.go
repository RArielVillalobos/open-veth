package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"open-veth/internal/config"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/vishvananda/netlink"
)

// setupIntegrationHandler creates a Handler wired to a real Docker daemon.
// If Docker is not available the test is skipped.
// track registers container names to be deleted during cleanup.
// cleanup deletes all tracked containers.
func setupIntegrationHandler(t *testing.T) (*Handler, *gin.Engine, *storage.MemoryRepository, *orchestrator.Manager, func(...string), func()) {
	t.Helper()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr, err := orchestrator.NewManager(logger)
	if err != nil {
		t.Fatalf("failed to create orchestrator manager: %v", err)
	}
	if err := mgr.TestConnection(ctx); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	repo := storage.NewMemoryRepository()
	cfg := &config.Config{}
	h := NewHandler(mgr, repo, logger, cfg)
	r := setupRouter(h)

	var created []string
	track := func(names ...string) {
		created = append(created, names...)
	}
	cleanup := func() {
		for _, n := range created {
			_ = mgr.DeleteNode(ctx, n)
		}
	}

	return h, r, repo, mgr, track, cleanup
}

// nodeName generates a unique container name for a test.
func nodeName(t *testing.T, suffix string) string {
	return fmt.Sprintf("it-%s-%s", t.Name(), suffix)
}

// labID generates a unique lab ID for a test.
func labID(t *testing.T) string {
	return fmt.Sprintf("it-lab-%s", t.Name())
}

// requireCleanDockerEnv skips the test if there are existing openveth containers.
// Reconcile tests use a fresh MemoryRepository and would classify any running
// openveth containers as zombies, deleting them. Run these tests only when no
// lab is active.
func requireCleanDockerEnv(t *testing.T, mgr *orchestrator.Manager) {
	t.Helper()
	existing, err := mgr.GetOpenVethContainers(context.Background())
	if err != nil {
		t.Skipf("could not list openveth containers: %v", err)
	}
	if len(existing) > 0 {
		t.Skip("skipping reconcile test: active openveth containers detected — stop all labs first")
	}
}

// containerExists checks whether a container with the given name exists in Docker.
func containerExists(t *testing.T, mgr *orchestrator.Manager, name string) bool {
	t.Helper()
	_, err := mgr.GetDockerClient().ContainerInspect(context.Background(), name)
	return err == nil
}

// containerRunning checks whether a container is running.
func containerRunning(t *testing.T, mgr *orchestrator.Manager, name string) bool {
	t.Helper()
	inspect, err := mgr.GetDockerClient().ContainerInspect(context.Background(), name)
	return err == nil && inspect.State.Running
}

// execInContainer runs a command inside a container and returns stdout + exit code.
func execInContainer(t *testing.T, mgr *orchestrator.Manager, containerName string, cmd []string) (string, int) {
	t.Helper()
	ctx := context.Background()
	cli := mgr.GetDockerClient()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execID, err := cli.ContainerExecCreate(ctx, containerName, execConfig)
	if err != nil {
		t.Fatalf("failed to create exec: %v", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		t.Fatalf("failed to attach exec: %v", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader); err != nil {
		t.Fatalf("failed to read exec output: %v", err)
	}

	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		t.Fatalf("failed to inspect exec: %v", err)
	}

	if errBuf.Len() > 0 {
		return outBuf.String() + "\n" + errBuf.String(), inspect.ExitCode
	}
	return outBuf.String(), inspect.ExitCode
}

// =============================================================================
// Node lifecycle
// =============================================================================

func TestIntegrationCreateAndDeleteNode(t *testing.T) {
	_, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "router")
	id := nodeName(t, "id")

	// Create node
	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	var created models.Node
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ContainerID == "" {
		t.Fatal("expected ContainerID after creation")
	}

	// Container must exist in Docker
	if !containerExists(t, mgr, name) {
		t.Error("container not found in Docker after CreateNode")
	}

	// Live interfaces must be readable
	w = doRequest(r, "GET", "/api/v1/nodes/"+id+"/interfaces", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for interfaces, got %d: %s", w.Code, w.Body.String())
	}
	var ifaces []models.InterfaceInfo
	json.Unmarshal(w.Body.Bytes(), &ifaces)
	if len(ifaces) == 0 {
		t.Error("expected at least one interface from live container")
	}

	// Delete node
	w = doRequest(r, "DELETE", "/api/v1/nodes/"+id, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Container must be gone
	if containerExists(t, mgr, name) {
		t.Error("container still exists in Docker after DeleteNode")
	}
}

// =============================================================================
// Reconciliation
// =============================================================================

func TestIntegrationReconcileRevivesNode(t *testing.T) {
	h, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()
	requireCleanDockerEnv(t, mgr)

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "reconcile")
	id := nodeName(t, "rec-id")

	// Create node via HTTP
	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	// Verify it exists
	if !containerExists(t, mgr, name) {
		t.Fatal("container not found after creation")
	}

	// Kill container directly (simulates crash or external stop)
	ctx := context.Background()
	if err := mgr.DeleteNode(ctx, name); err != nil {
		t.Fatalf("failed to kill container: %v", err)
	}

	// Container is gone but DB record remains
	if containerExists(t, mgr, name) {
		t.Fatal("container should be gone after direct kill")
	}

	// Reconcile must recreate it
	if err := h.ReconcileState(ctx); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if !containerExists(t, mgr, name) {
		t.Error("container was not revived by ReconcileState")
	}

	// RuntimeStore must reflect the new container
	state, ok := h.Runtime.Get(id)
	if !ok {
		t.Error("node missing from RuntimeStore after reconcile")
	}
	if state.ContainerID == "" {
		t.Error("expected non-empty ContainerID in RuntimeStore after reconcile")
	}
}

// =============================================================================
// Save state + snapshot
// =============================================================================

func TestIntegrationSaveLabStateSnapshot(t *testing.T) {
	_, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "snapshot")
	id := nodeName(t, "snap-id")

	// Create node
	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	// Save lab state (snapshots running containers)
	w = doRequest(r, "POST", "/api/v1/laboratories/"+lid+"/save-state", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["snapshots_saved"] != float64(1) {
		t.Errorf("expected 1 snapshot, got %v", resp["snapshots_saved"])
	}

	// Snapshot image must exist locally
	snapshotName := models.SnapshotImageName(id)
	ctx := context.Background()
	if !mgr.ImageExists(ctx, snapshotName) {
		t.Errorf("snapshot image %s not found in Docker", snapshotName)
	}

	// Cleanup snapshot image explicitly (normal cleanup only removes containers)
	defer mgr.RemoveImage(ctx, snapshotName)
}

// =============================================================================
// Laboratory activation
// =============================================================================

// requireNetlink skips the test if the kernel does not support veth pairs.
// This happens when running a Windows Go binary inside WSL2 — Docker API works,
// but direct netlink syscalls do not.
func requireNetlink(t *testing.T) {
	t.Helper()
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "veth-test-ok"},
		PeerName:  "veth-test-ok-peer",
	}
	if err := netlink.LinkAdd(veth); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not implemented") ||
			strings.Contains(msg, "operation not supported") ||
			strings.Contains(msg, "operation not permitted") {
			t.Skip("netlink veth pairs not available (missing CAP_NET_ADMIN or unsupported kernel)")
		}
		t.Fatalf("unexpected netlink error: %v", err)
	}
	_ = netlink.LinkDel(veth)
}

func TestIntegrationActivateLaboratory(t *testing.T) {
	requireNetlink(t)

	_, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Activate Test"})

	n1Name := nodeName(t, "host1")
	n2Name := nodeName(t, "host2")
	n1ID := nodeName(t, "id1")
	n2ID := nodeName(t, "id2")

	// Seed DB with topology (no containers yet)
	// Image must be set because ActivateLaboratory does not override it.
	repo.SaveNode(models.Node{ID: n1ID, Name: n1Name, Type: models.HOST, LabID: lid, Image: models.GetImageForType(models.HOST)})
	repo.SaveNode(models.Node{ID: n2ID, Name: n2Name, Type: models.HOST, LabID: lid, Image: models.GetImageForType(models.HOST)})
	repo.SaveLink(models.Link{
		ID: "link-1", LabID: lid,
		SourceID: n1ID, TargetID: n2ID,
		SourceInt: "eth1", TargetInt: "eth1",
		Enabled: true,
	})
	repo.SaveInterfaceConfigs(lid, []models.InterfaceConfig{
		{LabID: lid, NodeID: n1ID, Interface: "eth1", Address: "10.0.0.1/24"},
		{LabID: lid, NodeID: n2ID, Interface: "eth1", Address: "10.0.0.2/24"},
	})

	// Activate lab (async)
	w := doRequest(r, "POST", "/api/v1/laboratories/"+lid+"/activate", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	track(n1Name, n2Name)

	// Wait for activation to complete (poll Docker state)
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if containerRunning(t, mgr, n1Name) && containerRunning(t, mgr, n2Name) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !containerRunning(t, mgr, n1Name) {
		t.Fatal("host1 container not running after activation")
	}
	if !containerRunning(t, mgr, n2Name) {
		t.Fatal("host2 container not running after activation")
	}

	// Verify interfaces and IPs
	inspect1, err := mgr.GetDockerClient().ContainerInspect(ctx, n1Name)
	if err != nil {
		t.Fatalf("failed to inspect host1: %v", err)
	}
	inspect2, err := mgr.GetDockerClient().ContainerInspect(ctx, n2Name)
	if err != nil {
		t.Fatalf("failed to inspect host2: %v", err)
	}

	ifaces1, err := mgr.GetNodeInterfaces(ctx, inspect1.ID)
	if err != nil {
		t.Fatalf("failed to get host1 interfaces: %v", err)
	}
	ifaces2, err := mgr.GetNodeInterfaces(ctx, inspect2.ID)
	if err != nil {
		t.Fatalf("failed to get host2 interfaces: %v", err)
	}

	var foundIP1, foundIP2 bool
	for _, iface := range ifaces1 {
		if iface.Name == "eth1" {
			for _, addr := range iface.IPAddresses {
				if strings.HasPrefix(addr.Address, "10.0.0.1") {
					foundIP1 = true
					break
				}
			}
		}
	}
	for _, iface := range ifaces2 {
		if iface.Name == "eth1" {
			for _, addr := range iface.IPAddresses {
				if strings.HasPrefix(addr.Address, "10.0.0.2") {
					foundIP2 = true
					break
				}
			}
		}
	}
	if !foundIP1 {
		t.Error("host1 eth1 does not have expected IP 10.0.0.1/24")
	}
	if !foundIP2 {
		t.Error("host2 eth1 does not have expected IP 10.0.0.2/24")
	}

	// Verify connectivity: ping from host1 to host2
	out, code := execInContainer(t, mgr, n1Name, []string{"ping", "-c", "1", "-W", "2", "10.0.0.2"})
	if code != 0 {
		t.Fatalf("ping from host1 to host2 failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "1 packets received") && !strings.Contains(out, "1 received") {
		t.Errorf("ping output did not indicate success: %s", out)
	}
}

// =============================================================================
// Stop / Start node
// =============================================================================

func TestIntegrationStopAndStartNode(t *testing.T) {
	h, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "host")
	id := nodeName(t, "id")

	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	if !containerRunning(t, mgr, name) {
		t.Fatal("container not running after creation")
	}

	// Stop via API
	w = doRequest(r, "POST", "/api/v1/nodes/"+id+"/stop", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from StopNode, got %d: %s", w.Code, w.Body.String())
	}
	if containerRunning(t, mgr, name) {
		t.Error("container still running after StopNode")
	}

	// DB must reflect stopped status
	node, ok := repo.GetNode(id)
	if !ok {
		t.Fatal("node not found in repo after stop")
	}
	if node.Status != models.NodeStatusStopped {
		t.Errorf("expected Status=%q in DB, got %q", models.NodeStatusStopped, node.Status)
	}
	// RuntimeStore must still have the ContainerID (not cleared on stop)
	if state, ok := h.Runtime.Get(id); !ok || state.ContainerID == "" {
		t.Error("expected ContainerID to remain in RuntimeStore after stop")
	}

	// Start via API
	w = doRequest(r, "POST", "/api/v1/nodes/"+id+"/start", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from StartNode, got %d: %s", w.Code, w.Body.String())
	}
	if !containerRunning(t, mgr, name) {
		t.Error("container not running after StartNode")
	}

	// DB must reflect running status
	node, ok = repo.GetNode(id)
	if !ok {
		t.Fatal("node not found in repo after start")
	}
	if node.Status != models.NodeStatusRunning {
		t.Errorf("expected Status=%q in DB, got %q", models.NodeStatusRunning, node.Status)
	}
}

// =============================================================================
// Reconcile with stopped (not missing) container
// =============================================================================

func TestIntegrationReconcileWithStoppedContainer(t *testing.T) {
	h, r, repo, mgr, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()
	requireCleanDockerEnv(t, mgr)

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "host")
	id := nodeName(t, "id")

	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	// Stop the container directly (simulates Docker Desktop restart or host reboot)
	ctx := context.Background()
	if err := mgr.StopNode(ctx, name); err != nil {
		t.Fatalf("failed to stop container directly: %v", err)
	}
	if containerRunning(t, mgr, name) {
		t.Fatal("container should be stopped before reconcile")
	}
	// Container exists but is stopped — this is the branch under test in reconcileNodes
	if !containerExists(t, mgr, name) {
		t.Fatal("container should still exist (stopped, not deleted)")
	}

	// Reconcile must restart it (not recreate — the existing stopped container is reused)
	if err := h.ReconcileState(ctx); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if !containerRunning(t, mgr, name) {
		t.Error("container not running after reconcile of stopped container")
	}

	state, ok := h.Runtime.Get(id)
	if !ok {
		t.Error("node missing from RuntimeStore after reconcile")
	}
	if state.ContainerID == "" {
		t.Error("expected non-empty ContainerID in RuntimeStore after reconcile")
	}
}

// =============================================================================
// SaveLabState rejects labs with no running containers
// =============================================================================

func TestIntegrationSaveLabStateRejectsStoppedNodes(t *testing.T) {
	_, r, repo, _, track, cleanup := setupIntegrationHandler(t)
	defer cleanup()

	lid := labID(t)
	repo.SaveLaboratory(models.Laboratory{ID: lid, Name: "Integration Lab"})

	name := nodeName(t, "host")
	id := nodeName(t, "id")

	// Create node (running)
	w := doRequest(r, "POST", "/api/v1/nodes", models.Node{ID: id, Name: name, Type: models.HOST, LabID: lid})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	track(name)

	// Stop via API — sets Status=stopped in DB
	w = doRequest(r, "POST", "/api/v1/nodes/"+id+"/stop", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from StopNode, got %d: %s", w.Code, w.Body.String())
	}

	// SaveLabState must return 409: no running containers even though ContainerID is set
	w = doRequest(r, "POST", "/api/v1/laboratories/"+lid+"/save-state", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for lab with only stopped nodes, got %d: %s", w.Code, w.Body.String())
	}
}
