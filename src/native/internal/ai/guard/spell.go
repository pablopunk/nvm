package guard

import (
	"net/url"
	"path/filepath"
	"strings"
)

// expandPath undoes the spellings a model may use for a path: a leading @, "~" and "~/", file:// URLs and Unicode
// spaces. A path that matches none of them is returned as it was written.
func expandPath(raw, home string) string {
	normalized := strings.TrimPrefix(strings.Map(normalizedSpace, raw), "@")
	switch {
	case normalized == "~":
		return home
	case strings.HasPrefix(normalized, "~/"):
		return filepath.Join(home, normalized[2:])
	case strings.HasPrefix(normalized, "file://"):
		parsed, err := url.Parse(normalized)
		if err != nil {
			return normalized
		}
		return parsed.Path
	}
	return normalized
}

// normalizedSpace maps the Unicode space characters a path can carry to the ASCII space.
func normalizedSpace(r rune) rune {
	if r == 0x00A0 || (r >= 0x2000 && r <= 0x200A) || r == 0x202F || r == 0x205F || r == 0x3000 {
		return ' '
	}
	return r
}
