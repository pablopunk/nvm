package clipboard

import (
	"net/netip"
	"net/url"
	"strings"
	"unicode"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// SingleWebURL returns the normalized address when the whole text is exactly one public http or https URL,
// and "" otherwise. Hosts without a dot, localhost and .local names are refused, as are non-public IP
// literals, so a copied intranet address never triggers a preview. The public-address policy is shared with
// the network port (platform.IsPublicAddress).
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
		if !platform.IsPublicAddress(addr) {
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
