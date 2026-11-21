package files

import (
	"path/filepath"
	"strings"
)

// searchEnv carries what a query needs from its index besides the query itself.
type searchEnv struct {
	// home abbreviates display paths.
	home string
	// roots are the index roots. Ignore rules are matched relative to the root that contains a folder.
	roots []string
	// skipDefaults mirrors the index configuration: query-level ignore lists also get DefaultIgnores unless it is set.
	skipDefaults bool
}

// dirState caches, per folder of a snapshot, whether a query may read files in it.
const (
	dirUnknown uint8 = iota
	dirAllowed
	dirExcluded
)

// plan is a query compiled against one snapshot: extension and root filters become tables indexed by the
// snapshot's ids, so the per-entry work is a few array reads before any name is scored.
type plan struct {
	snap          *Snapshot
	env           searchEnv
	query         Query
	limit         int
	exts          []bool
	restrictRoots bool
	roots         []string
	rules         IgnoreRules
	// scorer is nil when the query has no text.
	scorer   *scorer
	dirState []uint8
	dirRel   []string
}

// newPlan compiles q against snap.
func newPlan(snap *Snapshot, q Query, env searchEnv) *plan {
	p := &plan{snap: snap, env: env, query: q, limit: effectiveLimit(q.Limit)}
	p.exts = extensionFilter(snap, q)
	if len(q.Roots) > 0 {
		p.restrictRoots = true
		p.roots = NormalizeRoots(env.home, q.Roots)
	}
	if len(q.Ignore) > 0 {
		p.rules = CompileIgnoreRules(q.Ignore, !env.skipDefaults)
	}
	if needle := lowerInto(nil, []byte(strings.TrimSpace(q.Text))); len(needle) > 0 {
		p.scorer = newScorer(needle)
	}
	p.dirState = make([]uint8, len(snap.dirs))
	if !p.rules.Empty() {
		p.dirRel = make([]string, len(snap.dirs))
	}
	return p
}

// extensionFilter returns, per extension id of snap, whether a query keeps files with that extension. It returns
// nil when the query restricts nothing by extension.
func extensionFilter(snap *Snapshot, q Query) []bool {
	wanted := map[string]bool{}
	for _, ext := range extensionsForKinds(q.Kinds) {
		wanted[ext] = true
	}
	for _, ext := range q.Extensions {
		if normalized := strings.TrimPrefix(strings.ToLower(ext), "."); normalized != "" {
			wanted[normalized] = true
		}
	}
	for _, pattern := range q.Patterns {
		if ext := patternExtension(pattern); ext != "" {
			wanted[ext] = true
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	filter := make([]bool, len(snap.exts))
	for i, ext := range snap.exts {
		filter[i] = wanted[ext]
	}
	return filter
}

// patternExtension returns the extension a pattern such as "*.pdf" names, lowercased, or "" when it names none.
func patternExtension(pattern string) string {
	dot := strings.LastIndexByte(pattern, '.')
	if dot < 0 {
		return ""
	}
	tail := pattern[dot+1:]
	if tail == "" {
		return ""
	}
	for i := 0; i < len(tail); i++ {
		c := tail[i]
		isAlnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlnum {
			return ""
		}
	}
	return strings.ToLower(tail)
}

// match reports whether entry i passes every filter and, when the query has text, its score.
func (p *plan) match(i int) (int, bool) {
	e := p.snap.entries[i]
	if p.exts != nil && !p.exts[e.ext] {
		return 0, false
	}
	if p.restrictRoots || !p.rules.Empty() {
		if !p.dirAllowed(e.dir) {
			return 0, false
		}
	}
	name := p.snap.nameOf(e)
	if !p.rules.Empty() && p.rules.Ignored(p.dirRel[e.dir], string(name), false) {
		return 0, false
	}
	if p.scorer == nil {
		return 0, true
	}
	score := p.scorer.score(name)
	return score, score > 0
}

// dirAllowed reports whether files in folder d may be returned, caching the answer for the rest of the query.
func (p *plan) dirAllowed(d uint32) bool {
	switch p.dirState[d] {
	case dirAllowed:
		return true
	case dirExcluded:
		return false
	}
	allowed := p.computeDirAllowed(d)
	if allowed {
		p.dirState[d] = dirAllowed
	} else {
		p.dirState[d] = dirExcluded
	}
	return allowed
}

// computeDirAllowed applies the root restriction and the ignore rules to folder d and every folder above it below its
// index root.
func (p *plan) computeDirAllowed(d uint32) bool {
	dir := p.snap.dirs[d]
	if p.restrictRoots && !anyRootContains(p.roots, dir) {
		return false
	}
	if p.rules.Empty() {
		return true
	}
	rel := ""
	if root := RootContaining(p.env.roots, dir); root != "" {
		if r, err := filepath.Rel(root, dir); err == nil && r != "." {
			rel = filepath.ToSlash(r)
		}
	}
	p.dirRel[d] = rel
	parent := ""
	if rel == "" {
		return true
	}
	for _, segment := range strings.Split(rel, "/") {
		if p.rules.Ignored(parent, segment, true) {
			return false
		}
		parent = joinRel(parent, segment)
	}
	return true
}

// anyRootContains reports whether dir is one of roots or lies beneath one of them.
func anyRootContains(roots []string, dir string) bool {
	for _, root := range roots {
		if IsInsideRoot(dir, root) {
			return true
		}
	}
	return false
}
