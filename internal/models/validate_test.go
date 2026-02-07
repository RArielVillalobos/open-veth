package models

import "testing"

func TestValidateInterfaceName(t *testing.T) {
	valid := []string{
		"eth0", "eth1", "br0", "mgmt0", "lo",
		"veth12345", "bond0", "wlan0",
		"eth0.100",   // VLAN sub-interface
		"br-custom",  // dashes allowed
		"a",          // single char
		"abcdefghijklmno", // 15 chars (max for Linux)
	}
	for _, name := range valid {
		if err := ValidateInterfaceName(name); err != nil {
			t.Errorf("expected %q to be valid, got: %v", name, err)
		}
	}

	invalid := []string{
		"",                  // empty
		"0eth",              // starts with digit
		"eth0; rm -rf /",    // command injection
		"eth0 && cat /etc/passwd", // command injection
		"eth0`id`",          // backtick injection
		"eth0$(whoami)",     // subshell injection
		"eth0|ls",           // pipe injection
		"abcdefghijklmnop",  // 16 chars (too long)
		"-eth0",             // starts with dash
		"eth 0",             // space
		"eth\t0",            // tab
		"eth\n0",            // newline
	}
	for _, name := range invalid {
		if err := ValidateInterfaceName(name); err == nil {
			t.Errorf("expected %q to be invalid, but it passed", name)
		}
	}
}

func TestValidateIPCIDR(t *testing.T) {
	valid := []string{
		"192.168.1.1/24",
		"10.0.0.1/8",
		"172.16.0.1/16",
		"0.0.0.0/0",
		"255.255.255.255/32",
		"fd00::1/64",          // IPv6
		"2001:db8::1/128",     // IPv6
	}
	for _, cidr := range valid {
		if err := ValidateIPCIDR(cidr); err != nil {
			t.Errorf("expected %q to be valid, got: %v", cidr, err)
		}
	}

	invalid := []string{
		"",
		"192.168.1.1",          // no prefix
		"not-an-ip/24",         // not an IP
		"192.168.1.1/24; rm -rf /", // injection
		"10.0.0.1/24 && cat /etc/shadow",
		"999.999.999.999/24",   // invalid octets
	}
	for _, cidr := range invalid {
		if err := ValidateIPCIDR(cidr); err == nil {
			t.Errorf("expected %q to be invalid, but it passed", cidr)
		}
	}
}

func TestValidateLinkID(t *testing.T) {
	valid := []string{
		"abcde",         // exactly 5
		"link-12345",    // longer
		"a1b2c3d4e5f6",  // UUID-like
	}
	for _, id := range valid {
		if err := ValidateLinkID(id); err != nil {
			t.Errorf("expected %q to be valid, got: %v", id, err)
		}
	}

	invalid := []string{
		"",     // empty
		"a",    // 1 char
		"abcd", // 4 chars
	}
	for _, id := range invalid {
		if err := ValidateLinkID(id); err == nil {
			t.Errorf("expected %q to be invalid, but it passed", id)
		}
	}
}
