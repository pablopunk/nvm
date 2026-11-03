package clipboard

import (
	"net/netip"
	"net/url"
	"strings"
	"unicode"
)

// nonPublicPrefixes are the ranges a link preview never contacts: loopback, private, link-local, carrier
// NAT, multicast, documentation and reserved space. They mirror the 1.x public-address policy.
var nonPublicPrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24",
	"203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "64:ff9b::/96", "100::/64", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
)

// mustPrefixes parses the constant prefix list. A bad entry is a programming error caught by the tests.
func mustPrefixes(list ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(list))
	for _, text := range list {
		out = append(out, netip.MustParsePrefix(text))
	}
	return out
}

// IsPublicAddress reports whether addr is a routable address a link preview may contact. An IPv4-mapped IPv6
// address is judged by its IPv4 form.
func IsPublicAddress(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// SingleWebURL returns the normalized address when the whole text is exactly one public http or https URL,
// and "" otherwise. Hosts without a dot, localhost and .local names are refused, as are non-public IP
// literals, so a copied intranet address never triggers a preview.
func SingleWebURL(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.IndexFunc(trimmed, unicode.IsSpace) >= 0 {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !IsPublicAddress(addr) {
			return ""
		}
	} else if host == "localhost" || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") {
		return ""
	}
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}
