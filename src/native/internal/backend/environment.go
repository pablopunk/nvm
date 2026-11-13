package backend

import (
	"fmt"
	"net/url"
	"strings"
)

// EnvironmentKind names which backend the app talks to. The kind decides which origins are allowed.
type EnvironmentKind string

// The environment kinds, matching the desktop app's NevermindEnvironment values.
const (
	// EnvironmentDevelopment is a backend on loopback, for unpackaged builds only.
	EnvironmentDevelopment EnvironmentKind = "development"
	// EnvironmentProduction is the public API at https://api.nvm.fyi.
	EnvironmentProduction EnvironmentKind = "production"
	// EnvironmentPRPreview is a pull-request preview deployment over HTTPS.
	EnvironmentPRPreview EnvironmentKind = "pr_preview"
	// EnvironmentCustom is any other HTTPS backend the owner typed in.
	EnvironmentCustom EnvironmentKind = "custom"
)

// Default origins for the kinds that have one.
const (
	productionBaseURL  = "https://api.nvm.fyi"
	developmentBaseURL = "http://localhost:4321"
	productionHost     = "api.nvm.fyi"
)

// Environment is a validated backend origin. Build one with NewEnvironment; the zero value is not usable.
type Environment struct {
	// Kind is the environment the origin belongs to.
	Kind EnvironmentKind
	// BaseURL is the origin with no path, query, fragment or credentials, and no trailing slash.
	BaseURL string
}

// NewEnvironment validates rawURL for kind. Production and development fall back to their default origins
// when rawURL is empty; the other kinds require one. packaged reports whether this is a packaged build, which
// refuses development and any loopback or private-network origin, as the desktop app does.
func NewEnvironment(kind EnvironmentKind, rawURL string, packaged bool) (Environment, error) {
	switch kind {
	case EnvironmentProduction:
		return buildEnvironment(kind, valueOr(rawURL, productionBaseURL), originPolicyHTTPS, packaged)
	case EnvironmentDevelopment:
		if packaged {
			return Environment{}, fmt.Errorf("backend: development is available only in unpackaged builds")
		}
		return buildEnvironment(kind, valueOr(rawURL, developmentBaseURL), originPolicyLoopback, packaged)
	case EnvironmentPRPreview, EnvironmentCustom:
		return buildEnvironment(kind, strings.TrimSpace(rawURL), originPolicyHTTPS, packaged)
	default:
		return Environment{}, fmt.Errorf("backend: unknown environment kind %q", kind)
	}
}

// valueOr returns value, or fallback when value is blank.
func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// originPolicy says what shape of origin a kind accepts.
type originPolicy int

const (
	// originPolicyHTTPS requires HTTPS and the default port.
	originPolicyHTTPS originPolicy = iota
	// originPolicyLoopback requires HTTP on a loopback host, any port.
	originPolicyLoopback
)

// buildEnvironment parses and checks an origin against policy, then applies the packaged-build rules.
func buildEnvironment(kind EnvironmentKind, rawURL string, policy originPolicy, packaged bool) (Environment, error) {
	origin, err := parseOrigin(rawURL, policy)
	if err != nil {
		return Environment{}, err
	}
	if kind == EnvironmentProduction && origin.host != productionHost {
		return Environment{}, fmt.Errorf("backend: production API origin must be %s", productionBaseURL)
	}
	if packaged && isUnsafeHost(origin.host) {
		return Environment{}, fmt.Errorf("backend: packaged builds cannot use localhost or private network addresses")
	}
	return Environment{Kind: kind, BaseURL: origin.baseURL}, nil
}

// parsedOrigin is an origin split into the parts the rules check.
type parsedOrigin struct {
	baseURL string
	host    string
}

// parseOrigin checks that rawURL is a bare origin with the scheme and port policy asked for.
func parseOrigin(rawURL string, policy originPolicy) (parsedOrigin, error) {
	value := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return parsedOrigin{}, fmt.Errorf("backend: enter a valid backend URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return parsedOrigin{}, fmt.Errorf("backend: backend URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return parsedOrigin{}, fmt.Errorf("backend: backend URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(value, "?#") {
		return parsedOrigin{}, fmt.Errorf("backend: backend URL must not include a query or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return parsedOrigin{}, fmt.Errorf("backend: backend URL must be a valid origin without a path")
	}
	host := strings.ToLower(parsed.Hostname())
	if policy == originPolicyLoopback {
		if parsed.Scheme != "http" || !isLoopbackHost(host) {
			return parsedOrigin{}, fmt.Errorf("backend: development origin must be HTTP on loopback")
		}
	} else {
		if parsed.Scheme != "https" {
			return parsedOrigin{}, fmt.Errorf("backend: backend URL must use HTTPS")
		}
		if parsed.Port() != "" {
			return parsedOrigin{}, fmt.Errorf("backend: backend URL must use the default HTTPS port")
		}
	}
	lowerHost := strings.ToLower(parsed.Host)
	return parsedOrigin{baseURL: parsed.Scheme + "://" + lowerHost, host: host}, nil
}

// isLoopbackHost reports whether host names this machine over the loopback interface.
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
