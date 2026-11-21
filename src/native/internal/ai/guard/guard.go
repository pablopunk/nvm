package guard

import (
	"path/filepath"
	"strings"
	"sync"
)

// Reasons the model reads when a path is refused. They name the rule and never the path.
const (
	CredentialReason = "access to credential locations is blocked"
	OutsideReason    = "access outside the allowed folder is blocked"
)

// Config is what a guard protects. Roots, when set, confine every path to one of them. ExtraDenied lists more
// locations to refuse, and Allowed carves exceptions out of ExtraDenied, never out of the home credential locations.
type Config struct {
	Home        string
	Roots       []string
	ExtraDenied []string
	Allowed     []string
}

// Decision is the verdict on one path. Reason is set when the path is refused.
type Decision struct {
	Allow  bool
	Reason string
}

// Guard checks paths against a Config. Its configuration is canonicalized once, on first use, so a root that is a
// symbolic link is compared by where it points.
type Guard struct {
	config       Config
	canonicalize func(path string) (string, error)
	once         sync.Once
	canon        canonical
}

// canonical is the configuration after symbolic links are resolved.
type canonical struct {
	home    string
	denied  []string
	allowed []string
	roots   []string
}

// New returns a guard for config. canonicalize resolves the symbolic links of an existing path and fails for a path that
// does not exist; the guard uses that to find the deepest existing ancestor of a path.
func New(config Config, canonicalize func(path string) (string, error)) *Guard {
	return &Guard{config: config, canonicalize: canonicalize}
}

// Check decides whether raw, a path the model named relative to cwd, may be used.
func (g *Guard) Check(raw, cwd string) Decision {
	current := g.state()
	if isEnvFile(expandPath(raw, current.home)) {
		return Decision{Reason: CredentialReason}
	}
	target := g.Resolve(raw, cwd)
	if isCredential(target, current) {
		return Decision{Reason: CredentialReason}
	}
	if len(current.roots) > 0 && !insideAny(fold(target), current.roots) {
		return Decision{Reason: OutsideReason}
	}
	return Decision{Allow: true}
}

// Resolve returns the absolute, symlink-resolved location of raw. A part of the path that does not exist yet is kept
// as written after its deepest existing ancestor.
func (g *Guard) Resolve(raw, cwd string) string {
	home := g.state().home
	return g.resolveExisting(lexicalPath(raw, cwd, home))
}

// Home returns the home folder the guard expands "~" with.
func (g *Guard) Home() string {
	return g.state().home
}

func (g *Guard) state() canonical {
	g.once.Do(func() {
		g.canon = canonical{
			home:    g.resolveExisting(absolute(g.config.Home)),
			denied:  g.resolveAll(g.config.ExtraDenied),
			allowed: g.resolveAll(g.config.Allowed),
			roots:   g.resolveAll(g.config.Roots),
		}
		g.canon.denied = append(g.canon.denied, g.resolveAll(homeCredentialRoots(g.canon.home))...)
	})
	return g.canon
}

func (g *Guard) resolveAll(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, g.resolveExisting(absolute(path)))
	}
	return out
}

// resolveExisting resolves the longest existing prefix of path and re-appends the missing tail. It returns path
// unchanged when nothing resolves.
func (g *Guard) resolveExisting(path string) string {
	var missing []string
	current := path
	for {
		if resolved, err := g.canonicalize(current); err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

func absolute(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// isCredential reports whether resolved names an env file, sits in a credential location, or sits in an extra denied
// location that no allowed location carves out.
func isCredential(resolved string, current canonical) bool {
	candidate := fold(resolved)
	if isEnvFile(candidate) {
		return true
	}
	if insideAny(candidate, homeCredentialRoots(current.home)) {
		return true
	}
	if insideAny(candidate, current.allowed) {
		return false
	}
	return insideAny(candidate, current.denied)
}

// lexicalPath makes raw absolute and cleans it. It does not touch the file system. A rooted path without a volume,
// such as \etc\hosts on Windows, is not absolute to Go, so it takes the process volume rather than joining the cwd.
func lexicalPath(raw, cwd, home string) string {
	expanded := expandPath(raw, home)
	switch {
	case filepath.IsAbs(expanded):
	case isRooted(expanded):
		expanded = absolute(expanded)
	default:
		expanded = filepath.Join(cwd, expanded)
	}
	return filepath.Clean(expanded)
}

// isRooted reports whether a path starts at a separator without naming a volume.
func isRooted(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`)
}

func homeCredentialRoots(home string) []string {
	roots := make([]string, 0, len(homeCredentialLocations))
	for _, location := range homeCredentialLocations {
		roots = append(roots, filepath.Join(home, location))
	}
	return roots
}

func insideAny(candidate string, roots []string) bool {
	for _, root := range roots {
		if inside(candidate, fold(filepath.Clean(root))) {
			return true
		}
	}
	return false
}

func inside(candidate, root string) bool {
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return candidate == root || strings.HasPrefix(candidate, prefix)
}

// fold lowers case, so a path compares the same on a case-insensitive file system.
func fold(value string) string {
	return strings.ToLower(value)
}
