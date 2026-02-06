package orchestrator

import (
	"fmt"
	"runtime"

	"open-veth/internal/models"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetworkManager handles kernel-level network configuration
type NetworkManager struct{}

// NewNetworkManager creates a new instance
func NewNetworkManager() *NetworkManager {
	return &NetworkManager{}
}

// =============================================================================
// Low-level helpers
// =============================================================================

// runInNs executes a function inside the network namespace of a process (PID).
// It handles thread locking and restores the original namespace on exit.
func (nm *NetworkManager) runInNs(pid int, action func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save original namespace
	origNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get original netns: %v", err)
	}
	defer origNs.Close()

	// Get target namespace handle
	targetNs, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("failed to get netns for pid %d: %v", pid, err)
	}
	defer targetNs.Close()

	// Switch to target namespace
	if err := netns.Set(targetNs); err != nil {
		return fmt.Errorf("failed to switch to netns for pid %d: %v", pid, err)
	}

	// Execute action
	actionErr := action()

	// CRITICAL: Restore original namespace before returning
	if err := netns.Set(origNs); err != nil {
		return fmt.Errorf("CRITICAL: failed to restore original netns (action error: %v): %v", actionErr, err)
	}

	return actionErr
}

// deleteIfExists removes a network interface if it exists (defensive cleanup)
func deleteIfExists(name string) {
	if link, _ := netlink.LinkByName(name); link != nil {
		_ = netlink.LinkDel(link)
	}
}

// createVethPair creates a veth pair on the host with the given names
func createVethPair(name1, name2 string, txQLen int) error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name1,
			TxQLen: txQLen,
		},
		PeerName: name2,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("failed to create veth pair (%s, %s): %v", name1, name2, err)
	}
	return nil
}

// moveToNamespace moves an interface from host to a container's network namespace
func moveToNamespace(ifaceName string, pid int) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found on host: %v", ifaceName, err)
	}

	targetNs, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("failed to get netns for pid %d: %v", pid, err)
	}
	defer targetNs.Close()

	if err := netlink.LinkSetNsFd(link, int(targetNs)); err != nil {
		return fmt.Errorf("failed to move interface %s to pid %d: %v", ifaceName, pid, err)
	}
	return nil
}

// renameAndBringUp renames an interface and brings it up.
// Must be called from within the target namespace (via runInNs).
func renameAndBringUp(currentName, newName string) error {
	link, err := netlink.LinkByName(currentName)
	if err != nil {
		return fmt.Errorf("interface %s not found after move: %v", currentName, err)
	}

	if err := netlink.LinkSetName(link, newName); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %v", currentName, newName, err)
	}

	// Re-fetch by new name
	link, err = netlink.LinkByName(newName)
	if err != nil {
		return fmt.Errorf("interface %s not found after rename: %v", newName, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up %s: %v", newName, err)
	}
	return nil
}

// =============================================================================
// Public API
// =============================================================================

// CreateLink creates a veth pair and connects two namespaces (PIDs)
func (nm *NetworkManager) CreateLink(link models.Link, pidSource, pidTarget int) error {
	if err := models.ValidateLinkID(link.ID); err != nil {
		return err
	}

	// Generate temporary names for host
	vethSource := fmt.Sprintf("veth%s_s", link.ID[:5])
	vethTarget := fmt.Sprintf("veth%s_t", link.ID[:5])

	// Defensive cleanup: remove stale interfaces if they exist
	deleteIfExists(vethSource)
	deleteIfExists(vethTarget)

	// 1. Create veth pair on host
	if err := createVethPair(vethSource, vethTarget, 1000); err != nil {
		return err
	}

	// 2. Move and configure source endpoint
	if err := nm.moveAndConfigure(vethSource, link.SourceInt, pidSource); err != nil {
		return fmt.Errorf("failed to configure source endpoint: %v", err)
	}

	// 3. Move and configure target endpoint
	if err := nm.moveAndConfigure(vethTarget, link.TargetInt, pidTarget); err != nil {
		return fmt.Errorf("failed to configure target endpoint: %v", err)
	}

	return nil
}

// moveAndConfigure moves an interface to a namespace and renames it
func (nm *NetworkManager) moveAndConfigure(hostIface, containerIface string, pid int) error {
	// Move interface from host to container namespace
	if err := moveToNamespace(hostIface, pid); err != nil {
		return err
	}

	// Configure inside namespace
	return nm.runInNs(pid, func() error {
		// Cleanup: delete zombie interface if it exists with target name
		deleteIfExists(containerIface)

		// Rename and bring up
		return renameAndBringUp(hostIface, containerIface)
	})
}

// CreateBridge creates a Linux Bridge on the host (acts as a Switch)
func (nm *NetworkManager) CreateBridge(bridgeName string) error {
	// Check if already exists (idempotent)
	if _, err := netlink.LinkByName(bridgeName); err == nil {
		return nil
	}

	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: bridgeName,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("failed to create bridge %s: %v", bridgeName, err)
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return fmt.Errorf("failed to bring up bridge %s: %v", bridgeName, err)
	}

	return nil
}

// ConnectNodeToBridge connects a container (PID) to a Bridge on the host
func (nm *NetworkManager) ConnectNodeToBridge(pid int, containerIface, bridgeName string) error {
	// Generate short names to avoid Linux's 15-char limit
	// Format: v<PID>-<first3chars>
	suffix := containerIface
	if len(suffix) > 3 {
		suffix = suffix[:3]
	}
	hostVeth := fmt.Sprintf("v%d-%s", pid, suffix)
	tempVeth := hostVeth + "c"

	// 1. Create veth pair
	if err := createVethPair(hostVeth, tempVeth, 0); err != nil {
		return err
	}

	// 2. Connect host side to bridge
	bridgeLink, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge %s not found: %v", bridgeName, err)
	}

	hostLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		return fmt.Errorf("host veth %s not found: %v", hostVeth, err)
	}

	if err := netlink.LinkSetMaster(hostLink, bridgeLink); err != nil {
		return fmt.Errorf("failed to attach %s to bridge %s: %v", hostVeth, bridgeName, err)
	}

	if err := netlink.LinkSetUp(hostLink); err != nil {
		return fmt.Errorf("failed to bring up %s: %v", hostVeth, err)
	}

	// 3. Move container side to namespace and configure
	if err := nm.moveAndConfigure(tempVeth, containerIface, pid); err != nil {
		return fmt.Errorf("failed to configure container interface: %v", err)
	}

	return nil
}

// SetInterfaceIP assigns an IP/CIDR to an interface inside a namespace (PID)
func (nm *NetworkManager) SetInterfaceIP(pid int, ifaceName string, ipCidr string) error {
	return nm.runInNs(pid, func() error {
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("interface %s not found: %v", ifaceName, err)
		}

		addr, err := netlink.ParseAddr(ipCidr)
		if err != nil {
			return fmt.Errorf("invalid IP format %s: %v", ipCidr, err)
		}

		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("failed to assign IP %s to %s: %v", ipCidr, ifaceName, err)
		}

		return nil
	})
}

// RemoveInterface deletes a network interface from a container namespace (PID)
func (nm *NetworkManager) RemoveInterface(pid int, ifaceName string) error {
	return nm.runInNs(pid, func() error {
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			// Interface not found = already gone = success
			return nil
		}
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete interface %s: %v", ifaceName, err)
		}
		return nil
	})
}
