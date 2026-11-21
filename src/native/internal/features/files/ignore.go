package files

import "strings"

// DefaultIgnores are the names the index skips unless the configuration turns them off, as ignore-patterns.ts
// lists them.
var DefaultIgnores = []string{"node_modules", ".git", "Library", "Applications"}

// IgnoreRules is a compiled list of ignore patterns in gitignore style. The last pattern that matches an entry
// decides: a plain pattern ignores the entry, a "!" pattern re-includes it. Rules:
//   - a pattern with no slash matches an entry's name at any depth;
//   - a pattern with a slash (other than a trailing one) is anchored to the root and matches the whole relative
//     path; a leading slash is allowed and ignored;
//   - a trailing slash makes the pattern match folders only;
//   - "*" matches any run of characters within one path segment, "**" also crosses segments, "?" matches one
//     character;
//   - matching is ASCII case-insensitive, which is the behavior of the platforms the index runs on;
//   - blank lines and lines starting with "#" are ignored.
//
// Patterns from the configuration are added to DefaultIgnores unless the caller skips the defaults. The zero
// value ignores nothing.
type IgnoreRules struct {
	rules       []ignoreRule
	usesRelPath bool
}

// ignoreRule is one parsed pattern.
type ignoreRule struct {
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool
}

// CompileIgnoreRules parses patterns into rules. withDefaults prepends DefaultIgnores.
func CompileIgnoreRules(patterns []string, withDefaults bool) IgnoreRules {
	var all []string
	if withDefaults {
		all = append(all, DefaultIgnores...)
	}
	all = append(all, patterns...)
	var rules IgnoreRules
	for _, raw := range all {
		rule, ok := parseIgnoreRule(raw)
		if !ok {
			continue
		}
		rules.rules = append(rules.rules, rule)
		rules.usesRelPath = rules.usesRelPath || rule.anchored
	}
	return rules
}

// parseIgnoreRule reads one pattern line. It reports false for blank and comment lines.
func parseIgnoreRule(raw string) (ignoreRule, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	var rule ignoreRule
	if strings.HasPrefix(line, "!") {
		rule.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		rule.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.Contains(line, "/") {
		rule.anchored = true
		line = strings.TrimPrefix(line, "/")
	}
	if line == "" {
		return ignoreRule{}, false
	}
	rule.pattern = line
	return rule, true
}

// Empty reports whether the rules ignore nothing, so callers can skip per-entry checks.
func (r IgnoreRules) Empty() bool { return len(r.rules) == 0 }

// Ignored reports whether the entry named name, inside the folder relDir (slash-separated, relative to its root,
// "" for the root itself), is ignored. isDir says whether the entry is a folder.
func (r IgnoreRules) Ignored(relDir, name string, isDir bool) bool {
	if len(r.rules) == 0 {
		return false
	}
	relPath := ""
	if r.usesRelPath {
		relPath = name
		if relDir != "" {
			relPath = relDir + "/" + name
		}
	}
	ignored := false
	for _, rule := range r.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		subject := name
		if rule.anchored {
			subject = relPath
		}
		if globMatch(rule.pattern, subject) {
			ignored = !rule.negate
		}
	}
	return ignored
}

// globMatch matches a pattern against a subject. Matching is ASCII case-insensitive. "*" does not cross a slash;
// "**" does; "?" matches one byte that is not a slash.
func globMatch(pattern, subject string) bool {
	if pattern == "" {
		return subject == ""
	}
	switch pattern[0] {
	case '*':
		crossing := len(pattern) > 1 && pattern[1] == '*'
		rest := pattern[1:]
		if crossing {
			rest = strings.TrimPrefix(pattern[2:], "/")
		}
		for i := 0; i <= len(subject); i++ {
			if globMatch(rest, subject[i:]) {
				return true
			}
			if i < len(subject) && !crossing && subject[i] == '/' {
				return false
			}
		}
		return false
	case '?':
		return len(subject) > 0 && subject[0] != '/' && globMatch(pattern[1:], subject[1:])
	default:
		return len(subject) > 0 && foldASCII(pattern[0]) == foldASCII(subject[0]) && globMatch(pattern[1:], subject[1:])
	}
}

// foldASCII lowercases an ASCII letter and leaves every other byte alone.
func foldASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
