package apps

import (
	"regexp"
	"strings"
)

// MaxBundleIDBytes is the longest bundle identifier the uninstall flow accepts.
const MaxBundleIDBytes = 255

// NeverMindBundleID is this app's identifier. Uninstall refuses to remove an app with it.
const NeverMindBundleID = "com.pablopunk.nvm"

const (
	asciiSpace  = 0x20
	asciiDelete = 0x7f
)

var bundleIDComponent = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// ValidateBundleID returns value and true when it is a safe reverse-DNS identifier: at most 255 bytes, no
// control characters, spaces, slashes or backslashes, and dot-separated components of letters, digits and
// hyphens. Anything else is refused before any path is built from it.
func ValidateBundleID(value string) (string, bool) {
	if value == "" || len(value) > MaxBundleIDBytes || hasUnsafeBundleCharacters(value) {
		return "", false
	}
	for _, part := range strings.Split(value, ".") {
		if !bundleIDComponent.MatchString(part) {
			return "", false
		}
	}
	return value, true
}

func hasUnsafeBundleCharacters(value string) bool {
	for _, r := range value {
		if r <= asciiSpace || r == asciiDelete || r == '/' || r == '\\' {
			return true
		}
	}
	return false
}

// BundleIDsOverlap reports whether two identifiers are equal or one is a dot-separated prefix of the other,
// ignoring case. An app whose ID overlaps another's may share its data, so such data is not selected by
// default.
func BundleIDsOverlap(left, right string) bool {
	l := strings.ToLower(left)
	r := strings.ToLower(right)
	return l == r || strings.HasPrefix(l, r+".") || strings.HasPrefix(r, l+".")
}
