package spike

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

// CredentialBlockReason is the refusal the model sees for a credential path. It is the 1.x text.
const CredentialBlockReason = "Access to credential locations is blocked."

// OutsideRootReason is the refusal for a path that resolves outside every configured root.
const OutsideRootReason = "Access outside the allowed workspace is blocked."

// homeCredentialLocations are the home-relative places a model must never read, from ai-credential-path-guard.ts.
var homeCredentialLocations = []string{
	".ssh",
	".aws",
	".gnupg",
	".config/gh",
	".config/gcloud",
	".azure",
	".netrc",
	".docker/config.json",
	".kube",
	".npmrc",
	".pypirc",
	".git-credentials",
	".pi",
	"Library/Keychains",
	"Library/Cookies",
	"Library/Safari",
	"Library/Application Support/Google/Chrome",
	"Library/Application Support/BraveSoftware",
	"Library/Application Support/Firefox",
	"Library/Application Support/Microsoft Edge",
	".config/google-chrome",
	".mozilla",
}

// envTemplateNames are dotenv files that hold placeholders, not secrets, and stay readable.
var envTemplateNames = map[string]bool{
	".env.example":  true,
	".env.sample":   true,
	".env.template": true,
	".env.dist":     true,
	".env.defaults": true,
}

// nevermindPrivateEntries are the Nevermind files under the user data directory that hold tokens and keys.
var nevermindPrivateEntries = []string{
	"nevermind-auth.json",
	"nevermind-auth-by-origin.json",
	"byo-key.json",
	"extension-secrets",
}

// CredentialPolicy is the input to IsCredentialPath. Allowed carve-outs win over ExtraDenied but never over the home
// credential locations, which are always denied.
type CredentialPolicy struct {
	HomeDir     string
	ExtraDenied []string
	Allowed     []string
}

// GuardConfig configures a Guard. Roots, when non-empty, confine every resolved path to one of them. The 1.x guard
// has no such root; the spike adds it because it is the stricter default WP2.4b should start from.
type GuardConfig struct {
	HomeDir     string
	ExtraDenied []string
	Allowed     []string
	Roots       []string
}

// Guard decides whether a tool path may be touched. Its configuration is canonicalized once, on first use, so a
// symlink into a credential location is judged by where it points.
type Guard struct {
	config   GuardConfig
	resolver Resolver
	once     sync.Once
	canon    canonicalGuard
}

type canonicalGuard struct {
	policy CredentialPolicy
	roots  []string
}

// NewGuard returns a guard that resolves symlinks through resolver.
func NewGuard(config GuardConfig, resolver Resolver) *Guard {
	return &Guard{config: config, resolver: resolver}
}

// Check decides whether the tool path raw, requested from working directory cwd, may be read.
func (g *Guard) Check(raw, cwd string) Decision {
	canon := g.canonical()
	if pathNamesEnvFile(expandToolPath(raw, canon.policy.HomeDir)) {
		return Decision{Reason: CredentialBlockReason}
	}
	target := g.Resolve(raw, cwd)
	if IsCredentialPath(target, canon.policy) {
		return Decision{Reason: CredentialBlockReason}
	}
	if len(canon.roots) > 0 && !isInsideAnyRoot(foldCase(target), canon.roots) {
		return Decision{Reason: OutsideRootReason}
	}
	return Decision{Allow: true}
}

// Resolve returns the absolute, symlink-resolved location of raw. Components that do not exist yet are kept as
// written after the deepest existing ancestor, so a path to a missing file still resolves.
func (g *Guard) Resolve(raw, cwd string) string {
	return g.canonicalizeExistingAncestor(lexicalPath(raw, cwd, g.canonical().policy.HomeDir))
}

// IsCredentialPath reports whether resolved names a credential file or sits inside a credential location.
func IsCredentialPath(resolved string, policy CredentialPolicy) bool {
	candidate := foldCase(resolved)
	if pathNamesEnvFile(candidate) {
		return true
	}
	if isInsideAnyRoot(candidate, homeCredentialRoots(policy.HomeDir)) {
		return true
	}
	if isInsideAnyRoot(candidate, policy.Allowed) {
		return false
	}
	return isInsideAnyRoot(candidate, policy.ExtraDenied)
}

// NevermindPrivatePaths lists the paths a model must not read in a Nevermind data directory: the agent state and
// the Nevermind secret files. The workspace is not among them.
func NevermindPrivatePaths(userDataDir, agentDir string) []string {
	paths := []string{agentDir}
	for _, entry := range nevermindPrivateEntries {
		paths = append(paths, filepath.Join(userDataDir, entry))
	}
	return paths
}

func (g *Guard) canonical() canonicalGuard {
	g.once.Do(func() { g.canon = g.canonicalize() })
	return g.canon
}

func (g *Guard) canonicalize() canonicalGuard {
	return canonicalGuard{
		policy: CredentialPolicy{
			HomeDir:     g.canonicalOrOriginal(g.config.HomeDir),
			ExtraDenied: append(g.canonicalAll(g.config.ExtraDenied), g.canonicalAll(homeCredentialRoots(g.config.HomeDir))...),
			Allowed:     g.canonicalAll(g.config.Allowed),
		},
		roots: g.canonicalAll(g.config.Roots),
	}
}

func (g *Guard) canonicalAll(paths []string) []string {
	canonical := make([]string, 0, len(paths))
	for _, path := range paths {
		canonical = append(canonical, g.canonicalOrOriginal(path))
	}
	return canonical
}

func (g *Guard) canonicalOrOriginal(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return g.canonicalizeExistingAncestor(absolute)
}

// canonicalizeExistingAncestor resolves the longest existing prefix of absolute through the resolver and re-appends
// the missing tail. It returns absolute unchanged when nothing resolves.
func (g *Guard) canonicalizeExistingAncestor(absolute string) string {
	var missing []string
	current := absolute
	for {
		if resolved, err := g.resolver.Canonical(current); err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolute
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

// expandToolPath undoes the spellings pi accepts for a path: a leading @, ~, ~/, file:// URLs and Unicode spaces.
func expandToolPath(raw, home string) string {
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
	default:
		return normalized
	}
}

func normalizedSpace(r rune) rune {
	if r == 0x00A0 || (r >= 0x2000 && r <= 0x200A) || r == 0x202F || r == 0x205F || r == 0x3000 {
		return ' '
	}
	return r
}

func lexicalPath(raw, cwd, home string) string {
	expanded := expandToolPath(raw, home)
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(cwd, expanded)
	}
	return filepath.Clean(expanded)
}

func homeCredentialRoots(home string) []string {
	roots := make([]string, 0, len(homeCredentialLocations))
	for _, location := range homeCredentialLocations {
		roots = append(roots, filepath.Join(home, location))
	}
	return roots
}

func isInsideAnyRoot(candidate string, roots []string) bool {
	for _, root := range roots {
		if isInsidePath(candidate, foldCase(filepath.Clean(root))) {
			return true
		}
	}
	return false
}

func isInsidePath(candidate, root string) bool {
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return candidate == root || strings.HasPrefix(candidate, prefix)
}

func foldCase(value string) string {
	return strings.ToLower(value)
}

func pathNamesEnvFile(path string) bool {
	return isEnvFileName(strings.ToLower(filepath.Base(path)))
}

func isEnvFileName(baseName string) bool {
	if envTemplateNames[baseName] {
		return false
	}
	return baseName == ".env" || strings.HasPrefix(baseName, ".env.")
}
