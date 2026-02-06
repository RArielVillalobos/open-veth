package models

import (
	"fmt"
	"net"
	"regexp"
)

// validIfaceName matches standard Linux interface names: alphanumeric, dashes, dots, up to 15 chars
var validIfaceName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{0,14}$`)

// ValidateInterfaceName ensures the name is a safe Linux interface identifier
func ValidateInterfaceName(name string) error {
	if !validIfaceName.MatchString(name) {
		return fmt.Errorf("invalid interface name %q: must match [a-zA-Z][a-zA-Z0-9._-]{0,14}", name)
	}
	return nil
}

// ValidateIPCIDR ensures the string is a valid IP/CIDR notation (e.g. "192.168.1.1/24")
func ValidateIPCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid IP/CIDR %q: %v", cidr, err)
	}
	return nil
}

// ValidateLinkID ensures the link ID is long enough for veth name generation
func ValidateLinkID(id string) error {
	if len(id) < 5 {
		return fmt.Errorf("link ID %q is too short: must be at least 5 characters", id)
	}
	return nil
}
