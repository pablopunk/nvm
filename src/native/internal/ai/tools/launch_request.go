package tools

import (
	"net/url"
	"regexp"
	"strings"
)

// launchRequest is the one link or app the user's current message asks to open in plain words.
type launchRequest struct {
	url string
	app string
}

// plainLaunchRequest is a whole message that only asks to open one thing, such as "open github.com" or "Open Safari.".
// A message that says anything more, such as a negation or a second instruction, does not match.
var plainLaunchRequest = regexp.MustCompile(`(?i)^(?:please\s+)?(?:open|launch|start|visit|go to)\s+([^\s,;!"'` + "`" + `]+(?: [^\s,;!"'` + "`" + `]+){0,3}?)(?:\s+please)?$`)

// webAddressWithoutScheme is a host name with at least one dot, an optional port and an optional path or query.
var webAddressWithoutScheme = regexp.MustCompile(`(?i)^[a-z0-9-]+(?:\.[a-z0-9-]+)+(?::\d+)?(?:[/?#]\S*)?$`)

// launchRequestIn reads the user's current message. It reports false for anything but one plain request to open one
// link or app.
func launchRequestIn(message string) (launchRequest, bool) {
	match := plainLaunchRequest.FindStringSubmatch(withoutClosingPunctuation(message))
	if match == nil {
		return launchRequest{}, false
	}
	target := match[1]
	if address, isAddress := requestedWebAddress(target); isAddress {
		normalized, ok := normalizedWebAddress(address)
		return launchRequest{url: normalized}, ok
	}
	return launchRequest{app: target}, true
}

func withoutClosingPunctuation(message string) string {
	return strings.TrimRight(strings.TrimSpace(message), ".!")
}

// requestedWebAddress is the target as a full web address when it names one; a bare host gets https.
func requestedWebAddress(target string) (string, bool) {
	if strings.Contains(target, "://") {
		return target, true
	}
	if strings.HasSuffix(strings.ToLower(target), ".app") || !webAddressWithoutScheme.MatchString(target) {
		return "", false
	}
	return "https://" + target, true
}

// normalizedWebAddress is the form two addresses are compared in: scheme and host in lower case, the default port
// dropped, an empty path as "/", and the query and fragment as written. An address with a user name, or that is not
// http or https, has no normalized form, so it never matches.
func normalizedWebAddress(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" && !isDefaultPort(scheme, port) {
		host += ":" + port
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	normalized := scheme + "://" + host + path
	if parsed.RawQuery != "" || parsed.ForceQuery {
		normalized += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		normalized += "#" + parsed.EscapedFragment()
	}
	return normalized, true
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}
