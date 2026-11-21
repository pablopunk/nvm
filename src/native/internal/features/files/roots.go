package files

import (
	"path/filepath"
	"strings"
)

// defaultRootNames are the personal folders indexed when no roots are configured, as file-index-roots.ts lists them.
var defaultRootNames = []string{"Desktop", "Documents", "Downloads", "Pictures", "Videos", "Movies"}

// DefaultRoots returns the standard personal folders under home. home is passed in rather than read from the
// environment so the feature stays free of OS reads.
func DefaultRoots(home string) []string {
	roots := make([]string, 0, len(defaultRootNames))
	for _, name := range defaultRootNames {
		roots = append(roots, filepath.Join(home, name))
	}
	return roots
}

// NormalizeRoots expands a leading "~" with home, drops relative and empty entries and cleans the rest. The
// result keeps the caller's order and drops duplicates.
func NormalizeRoots(home string, roots []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range roots {
		expanded := ExpandUserPath(home, root)
		if expanded == "" || !filepath.IsAbs(expanded) {
			continue
		}
		cleaned := filepath.Clean(expanded)
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
	}
	return out
}

// ExpandUserPath replaces a leading "~" or "~/" with home. Any other value is returned unchanged.
func ExpandUserPath(home, value string) string {
	switch {
	case value == "~":
		return home
	case strings.HasPrefix(value, "~/"):
		return filepath.Join(home, value[2:])
	default:
		return value
	}
}

// DisplayPath abbreviates a path under home to "~" form for display, and leaves every other path unchanged.
func DisplayPath(home, p string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// IsInsideRoot reports whether p is root itself or lies beneath it. Both arguments must be clean paths.
func IsInsideRoot(p, root string) bool {
	if p == root {
		return true
	}
	if strings.HasSuffix(root, string(filepath.Separator)) {
		return strings.HasPrefix(p, root)
	}
	return strings.HasPrefix(p, root+string(filepath.Separator))
}

// RootContaining returns the longest of roots that contains p, or "" when none does.
func RootContaining(roots []string, p string) string {
	best := ""
	for _, root := range roots {
		if IsInsideRoot(p, root) && len(root) > len(best) {
			best = root
		}
	}
	return best
}
