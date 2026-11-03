package api

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is the API version a manifest targets. It is written as "major.minor" in JSON.
// Patch numbers are not part of the contract.
type Version struct {
	Major int
	Minor int
}

// Current is the API version implemented by this build of the host.
var Current = Version{Major: 1, Minor: 0}

// ParseVersion reads a "major.minor" string.
func ParseVersion(text string) (Version, error) {
	majorText, minorText, found := strings.Cut(text, ".")
	if !found {
		return Version{}, fmt.Errorf("api version %q: want major.minor", text)
	}
	major, err := strconv.Atoi(majorText)
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("api version %q: major must be a non-negative integer", text)
	}
	minor, err := strconv.Atoi(minorText)
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("api version %q: minor must be a non-negative integer", text)
	}
	return Version{Major: major, Minor: minor}, nil
}

// String formats the version as "major.minor".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// CheckCompatibility decides whether a host implementing host can load a manifest that targets
// target. Majors must match. A manifest may target any minor up to the host's minor; a newer
// minor is rejected with a message telling the user to update Nevermind. Unknown capabilities,
// trigger types, action kinds and JSON fields never fail compatibility: they degrade with a
// diagnostic so that newer extensions still load on older hosts.
func CheckCompatibility(target string, host Version) error {
	version, err := ParseVersion(target)
	if err != nil {
		return err
	}
	switch {
	case version.Major != host.Major:
		return fmt.Errorf("extension targets API %s; this Nevermind supports %s", version, host)
	case version.Minor > host.Minor:
		return fmt.Errorf("extension targets API %s, newer than this Nevermind (%s); update Nevermind", version, host)
	}
	return nil
}
