package backend

import (
	"net/netip"
	"strings"
)

// unsafePrefixes are address ranges a packaged build must not reach that net/netip does not classify as
// private or loopback on its own: unspecified and "this network" space, carrier-grade NAT, benchmarking and
// reserved space.
var unsafePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// isUnsafeHost reports whether host is localhost, or an IP literal in loopback, private, link-local,
// unspecified, multicast, shared, benchmarking or reserved space. It classifies names and literals only; it
// does not resolve DNS, so a public name that resolves privately is not caught here.
func isUnsafeHost(host string) bool {
	normalized := strings.TrimSuffix(strings.Trim(strings.ToLower(host), "[]"), ".")
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(normalized)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	for _, prefix := range unsafePrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
