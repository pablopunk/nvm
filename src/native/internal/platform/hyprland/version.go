package hyprland

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Version is a Hyprland release number.
type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// AtLeast reports whether v is o or later.
func (v Version) AtLeast(o Version) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch >= o.Patch
}

// ParseVersion reads "0.55.1", "v0.55.1" or "v0.55.1-12-gabcdef".
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("hyprland: version %q is not a release number", s)
	}
	var nums [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("hyprland: version %q is not a release number", s)
		}
		nums[i] = n
	}
	return Version{nums[0], nums[1], nums[2]}, nil
}

// parseVersionJSON reads the output of "hyprctl version -j": its "version" field, or its "tag" on older releases.
func parseVersionJSON(out []byte) (Version, error) {
	var info struct {
		Version string `json:"version"`
		Tag     string `json:"tag"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return Version{}, fmt.Errorf("hyprland: hyprctl version gave no JSON: %w", err)
	}
	if v, err := ParseVersion(info.Version); err == nil {
		return v, nil
	}
	return ParseVersion(info.Tag)
}
