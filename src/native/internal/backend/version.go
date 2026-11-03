package backend

import (
	"regexp"
	"strconv"
	"strings"
)

// strictVersionPattern matches the version forms the release pipeline produces: major, optional minor and
// patch, optional pre-release, optional build metadata, with an optional leading v.
var strictVersionPattern = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// parsedVersion is a version split into its numeric core and its pre-release identifiers.
type parsedVersion struct {
	core       [3]uint64
	prerelease []string
}

// parseVersion splits a version string. It reports false for anything that is not a release-shaped version.
func parseVersion(text string) (parsedVersion, bool) {
	match := strictVersionPattern.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return parsedVersion{}, false
	}
	var parsed parsedVersion
	for i := 0; i < 3; i++ {
		if match[i+1] == "" {
			continue
		}
		value, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return parsedVersion{}, false
		}
		parsed.core[i] = value
	}
	if match[4] != "" {
		parsed.prerelease = strings.Split(match[4], ".")
	}
	return parsed, true
}

// isNewerVersion reports whether candidate sorts after current under semver precedence. The second result is
// false when either version cannot be parsed, in which case callers must not treat the candidate as newer.
func isNewerVersion(candidate, current string) (bool, bool) {
	left, leftOK := parseVersion(candidate)
	right, rightOK := parseVersion(current)
	if !leftOK || !rightOK {
		return false, false
	}
	return compareVersions(left, right) > 0, true
}

// compareVersions orders two parsed versions: the numeric core first, then the pre-release, where a release
// outranks its pre-releases.
func compareVersions(left, right parsedVersion) int {
	for i := range left.core {
		if diff := compareUint(left.core[i], right.core[i]); diff != 0 {
			return diff
		}
	}
	return comparePrerelease(left.prerelease, right.prerelease)
}

// comparePrerelease implements semver pre-release precedence.
func comparePrerelease(left, right []string) int {
	switch {
	case len(left) == 0 && len(right) == 0:
		return 0
	case len(left) == 0:
		return 1
	case len(right) == 0:
		return -1
	}
	for i := 0; i < len(left) && i < len(right); i++ {
		if diff := comparePrereleaseIdentifier(left[i], right[i]); diff != 0 {
			return diff
		}
	}
	return compareInt(len(left), len(right))
}

// comparePrereleaseIdentifier compares one pre-release identifier: numeric identifiers compare as numbers and
// rank below alphanumeric ones.
func comparePrereleaseIdentifier(left, right string) int {
	leftNumber, leftErr := strconv.ParseUint(left, 10, 64)
	rightNumber, rightErr := strconv.ParseUint(right, 10, 64)
	switch {
	case leftErr == nil && rightErr == nil:
		return compareUint(leftNumber, rightNumber)
	case leftErr == nil:
		return -1
	case rightErr == nil:
		return 1
	default:
		return strings.Compare(left, right)
	}
}

// compareUint returns -1, 0 or 1.
func compareUint(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// compareInt returns -1, 0 or 1.
func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
