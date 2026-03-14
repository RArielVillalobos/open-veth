package models

import "testing"

func TestGetImageForType(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		expected string
	}{
		{ROUTER, ImgRouter},
		{HOST, ImgHost},
		{SWITCH, ImgHost},
		{HUB, ImgHost},                 // Hub uses host image
		{CLOUD, ImgHost},               // Cloud uses host image
		{LINUX, ImgLinux},              // Linux uses its own debian image
		{NodeType("unknown"), ImgHost}, // default fallback
	}

	for _, tt := range tests {
		got := GetImageForType(tt.nodeType)
		if got != tt.expected {
			t.Errorf("GetImageForType(%q) = %q, want %q", tt.nodeType, got, tt.expected)
		}
	}
}

func TestFormatMAC(t *testing.T) {
	tests := []struct {
		name     string
		mac      []byte
		expected string
	}{
		{
			name:     "standard MAC",
			mac:      []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			expected: "aa:bb:cc:dd:ee:ff",
		},
		{
			name:     "all zeros",
			mac:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expected: "00:00:00:00:00:00",
		},
		{
			name:     "broadcast",
			mac:      []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			expected: "ff:ff:ff:ff:ff:ff",
		},
		{
			name:     "short input returns default",
			mac:      []byte{0xaa, 0xbb},
			expected: "00:00:00:00:00:00",
		},
		{
			name:     "nil input returns default",
			mac:      nil,
			expected: "00:00:00:00:00:00",
		},
		{
			name:     "longer than 6 bytes uses first 6",
			mac:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			expected: "01:02:03:04:05:06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMAC(tt.mac)
			if got != tt.expected {
				t.Errorf("FormatMAC(%v) = %q, want %q", tt.mac, got, tt.expected)
			}
		})
	}
}
