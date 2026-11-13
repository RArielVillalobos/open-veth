//go:build linux

package orchestrator

import (
	"net"
	"os"
	"runtime"
	"testing"

	"open-veth/internal/models"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func testNamespacePID(t *testing.T) int {
	t.Helper()
	type result struct {
		pid int
		err error
	}
	ready := make(chan result)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		original, err := netns.Get()
		if err != nil {
			ready <- result{err: err}
			runtime.UnlockOSThread()
			return
		}
		defer original.Close()
		ns, err := netns.New()
		if err != nil {
			ready <- result{err: err}
			runtime.UnlockOSThread()
			return
		}
		defer ns.Close()
		ready <- result{pid: unix.Gettid()}
		<-stop
		// A thread whose namespace cannot be restored must never rejoin the pool.
		if netns.Set(original) == nil {
			runtime.UnlockOSThread()
		}
	}()
	state := <-ready
	if state.err != nil {
		<-done
		if os.Getenv("REQUIRE_NETWORK_TESTS") == "1" {
			t.Fatal(state.err)
		}
		t.Skipf("network namespaces require CAP_SYS_ADMIN: %v", state.err)
	}
	t.Cleanup(func() { close(stop); <-done })
	return state.pid
}

func TestNetworkRestoresDisabledLink(t *testing.T) {
	source, target := testNamespacePID(t), testNamespacePID(t)
	nm := NewNetworkManager()
	link := models.Link{ID: "nwdis-link", SourceInt: "eth0", TargetInt: "eth0", Enabled: false}
	if err := nm.CreateLink(link, source, target); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{source, target} {
		if err := nm.runInNs(pid, func() error {
			iface, err := netlink.LinkByName("eth0")
			if err != nil {
				return err
			}
			if iface.Attrs().Flags&net.FlagUp != 0 {
				t.Error("disabled link was brought up")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNetworkNewLinkPreservesOccupiedInterface(t *testing.T) {
	source, target := testNamespacePID(t), testNamespacePID(t)
	nm := NewNetworkManager()
	link := models.Link{ID: "nwocc-link", SourceInt: "eth0", TargetInt: "eth0", Enabled: true}
	if err := nm.CreateNewLink(link, source, target); err != nil {
		t.Fatal(err)
	}
	if err := nm.SetInterfaceIP(source, "eth0", "10.0.0.1/24"); err != nil {
		t.Fatal(err)
	}
	link.ID = "nwnew-link"
	if err := nm.CreateNewLink(link, source, target); err == nil {
		t.Fatal("occupied interface must be rejected")
	}
	if err := nm.runInNs(source, func() error {
		iface, err := netlink.LinkByName("eth0")
		if err != nil {
			return err
		}
		addresses, err := netlink.AddrList(iface, netlink.FAMILY_V4)
		if err == nil && (len(addresses) != 1 || addresses[0].IP.String() != "10.0.0.1") {
			t.Errorf("existing IP was lost: %v", addresses)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
