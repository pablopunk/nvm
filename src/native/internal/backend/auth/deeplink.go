package auth

import (
	"errors"
	"net/url"
	"strings"
)

// Scheme is the URL scheme the app claims. The backend's approval page redirects to nvm://auth?code=...
const Scheme = "nvm"

// authHost is the host of the sign-in link. Other hosts of the scheme are not sign-in links.
const authHost = "auth"

// ErrNotAuthLink reports a URL that is not a sign-in link, so the handler ignores it.
var ErrNotAuthLink = errors.New("auth: not a sign-in link")

// ErrMissingCode reports a sign-in link without a code.
var ErrMissingCode = errors.New("auth: sign-in link has no code")

// ErrTokenLink reports a link whose code is an access token. Tokens never travel in links.
var ErrTokenLink = errors.New("auth: sign-in link carries an access token")

// DeepLink is a parsed sign-in link.
type DeepLink struct {
	// Code is the one-time device code to exchange for a token.
	Code string
	// BaseURL is the origin the code belongs to: the link's origin when it is one the app trusts, otherwise the
	// active origin.
	BaseURL string
}

// ParseDeepLink reads a sign-in link. activeBaseURL is the origin the app is connected to. A link that names an
// origin the app does not trust is still accepted, but resolves to activeBaseURL, so a link cannot move the user
// to a backend of someone else's choosing.
func ParseDeepLink(rawURL string, activeBaseURL string) (DeepLink, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != Scheme || parsed.Host != authHost {
		return DeepLink{}, ErrNotAuthLink
	}
	query := parsed.Query()
	code := query.Get("code")
	if code == "" {
		return DeepLink{}, ErrMissingCode
	}
	if isAccessTokenShaped(code) {
		return DeepLink{}, ErrTokenLink
	}
	return DeepLink{Code: code, BaseURL: resolveLinkOrigin(query.Get("base_url"), activeBaseURL)}, nil
}

// isAccessTokenShaped reports whether code looks like a personal access token or a JWT, which the app refuses
// to exchange.
func isAccessTokenShaped(code string) bool {
	return strings.HasPrefix(code, "nvm_") || strings.HasPrefix(code, "eyJ")
}

// resolveLinkOrigin returns the origin a link names when the app trusts it, and the active origin otherwise.
// Trusted origins are production, loopback development, and the origin the app already uses.
func resolveLinkOrigin(encoded string, activeBaseURL string) string {
	candidate := strings.TrimSuffix(strings.TrimSpace(encoded), "/")
	if candidate == "" {
		return activeBaseURL
	}
	trusted := candidate == productionOrigin || candidate == developmentOrigin || candidate == strings.TrimSuffix(activeBaseURL, "/")
	if trusted {
		return candidate
	}
	return activeBaseURL
}
