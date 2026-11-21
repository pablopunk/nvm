package platform

import (
	"net/netip"
	"testing"
)

// TestIsPublicAddressRefusesNonRoutableRanges covers the SSRF vectors a link preview must never reach:
// loopback, private, link-local and metadata addresses, and their IPv6 and mapped forms.
func TestIsPublicAddressRefusesNonRoutableRanges(t *testing.T) {
	refused := []string{
		"127.0.0.1", "127.8.9.10", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "240.0.0.1",
		"::1", "::", "fe80::1", "fc00::1", "fd12:3456::1", "::ffff:127.0.0.1", "::ffff:10.0.0.5",
		"64:ff9b::808:808",
	}
	for _, text := range refused {
		addr := netip.MustParseAddr(text)
		if IsPublicAddress(addr) {
			t.Errorf("%s is public, want refused", text)
		}
	}
}

// TestIsPublicAddressAcceptsRoutableAddresses checks that ordinary public addresses pass, including the
// edges of the private ranges.
func TestIsPublicAddressAcceptsRoutableAddresses(t *testing.T) {
	allowed := []string{"8.8.8.8", "1.1.1.1", "172.15.255.255", "172.32.0.1", "192.169.0.1", "2606:4700::1111", "::ffff:8.8.8.8"}
	for _, text := range allowed {
		if !IsPublicAddress(netip.MustParseAddr(text)) {
			t.Errorf("%s is refused, want public", text)
		}
	}
	if IsPublicAddress(netip.Addr{}) {
		t.Error("the zero address is public")
	}
}
