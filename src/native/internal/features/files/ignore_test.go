package files

import (
	"path/filepath"
	"testing"
)

// ignoreCase is one entry checked against compiled rules.
type ignoreCase struct {
	name     string
	relDir   string
	entry    string
	isDir    bool
	ignored  bool
	patterns []string
	defaults bool
}

// TestIgnoreRulesTable covers the gitignore-style syntax the index accepts: name and anchored patterns, folder-only
// patterns, wildcards, negation, comments and the default list.
func TestIgnoreRulesTable(t *testing.T) {
	cases := []ignoreCase{
		{name: "default node_modules folder", entry: "node_modules", isDir: true, defaults: true, ignored: true},
		{name: "default .git folder", entry: ".git", isDir: true, defaults: true, ignored: true},
		{name: "default Library folder", entry: "Library", isDir: true, defaults: true, ignored: true},
		{name: "default Applications file", entry: "Applications", defaults: true, ignored: true},
		{name: "a name that only starts like a default", entry: "Libraries", isDir: true, defaults: true},
		{name: "defaults can be skipped", entry: "node_modules", isDir: true, defaults: false},
		{name: "wildcard matches a name", entry: "scratch.tmp", patterns: []string{"*.tmp"}, ignored: true},
		{name: "wildcard is ASCII case-insensitive", entry: "SCRATCH.TMP", patterns: []string{"*.tmp"}, ignored: true},
		{name: "wildcard does not match a longer name", entry: "scratch.tmp.bak", patterns: []string{"*.tmp"}},
		{name: "question mark matches one character", entry: "a1.log", patterns: []string{"a?.log"}, ignored: true},
		{name: "question mark needs exactly one character", entry: "a12.log", patterns: []string{"a?.log"}},
		{name: "folder-only pattern matches a folder", entry: "build", isDir: true, patterns: []string{"build/"}, ignored: true},
		{name: "folder-only pattern skips a file of the same name", entry: "build", patterns: []string{"build/"}},
		{name: "anchored pattern matches at the root", entry: "docs", isDir: true, patterns: []string{"/docs"}, ignored: true},
		{name: "anchored pattern skips deeper folders", relDir: "a", entry: "docs", isDir: true, patterns: []string{"/docs"}},
		{name: "pattern with a slash is anchored", relDir: "a", entry: "b", isDir: true, patterns: []string{"a/b"}, ignored: true},
		{name: "double star crosses folders", relDir: "x/y", entry: "cache", isDir: true, patterns: []string{"**/cache"}, ignored: true},
		{name: "star inside an anchored pattern matches one segment", relDir: "x", entry: "cache", isDir: true, patterns: []string{"x/*"}, ignored: true},
		{name: "star does not cross a slash", relDir: "x/y", entry: "cache", isDir: true, patterns: []string{"x/*"}},
		{name: "negation re-includes a later match", entry: "keep.log", patterns: []string{"*.log", "!keep.log"}},
		{name: "last matching rule decides", entry: "x.log", patterns: []string{"!x.log", "*.log"}, ignored: true},
		{name: "comments and blank lines are ignored", entry: "notes.txt", patterns: []string{"# nothing", "  "}},
		{name: "surrounding spaces are trimmed", entry: "scratch.tmp", patterns: []string{"  *.tmp  "}, ignored: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rules := CompileIgnoreRules(c.patterns, c.defaults)
			if got := rules.Ignored(c.relDir, c.entry, c.isDir); got != c.ignored {
				t.Fatalf("Ignored(%q, %q, dir=%v) = %v, want %v", c.relDir, c.entry, c.isDir, got, c.ignored)
			}
		})
	}
}

// TestEmptyIgnoreRulesIgnoreNothing checks the zero value, which the query path uses when no rules apply.
func TestEmptyIgnoreRulesIgnoreNothing(t *testing.T) {
	var rules IgnoreRules
	if !rules.Empty() || rules.Ignored("", "node_modules", true) {
		t.Fatal("the zero IgnoreRules must ignore nothing")
	}
}

// TestGlobMatchStarsAndQuestionMarksOnlyMatchWhatTheyShould checks the matcher directly on the edge cases.
func TestGlobMatchStarsAndQuestionMarksOnlyMatchWhatTheyShould(t *testing.T) {
	cases := []struct {
		pattern, subject string
		want             bool
	}{
		{"", "", true},
		{"", "x", false},
		{"*", "", true},
		{"*", "a/b", false},
		{"**", "a/b", true},
		{"a?c", "a/c", false},
		{"**/x", "x", true},
		{"**/x", "p/q/x", true},
		{"ab", "AB", true},
		{"ab", "abc", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.subject); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.subject, got, c.want)
		}
	}
}

// TestHiddenAndDepthAdmission ports the admission cases of file-index-watcher.test.ts ("changes inside the indexed depth
// can affect the file index" and "changes below the indexed depth, hidden, or ignored paths are skipped") against the
// index's admit rule.
func TestHiddenAndDepthAdmission(t *testing.T) {
	ix := &Index{cfg: Config{MaxDepth: 2}, rules: CompileIgnoreRules(nil, true)}
	cases := []struct {
		name  string
		rel   string
		isDir bool
		want  bool
	}{
		{name: "file at the root", rel: "a.pdf", want: true},
		{name: "file two folders deep", rel: filepath.ToSlash(filepath.Join("x", "y", "b.png")), want: true},
		{name: "file three folders deep is below the limit", rel: "x/y/z/deep.txt", want: false},
		{name: "hidden file", rel: ".DS_Store", want: false},
		{name: "file inside an ignored folder", rel: "app/node_modules/a", want: false},
		{name: "folder at the depth limit", rel: "x/y", isDir: true, want: true},
		{name: "folder below the depth limit", rel: "x/y/z", isDir: true, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ix.admit(c.rel, c.isDir); got != c.want {
				t.Fatalf("admit(%q, dir=%v) = %v, want %v", c.rel, c.isDir, got, c.want)
			}
		})
	}
	ix.cfg.IncludeHidden = true
	if !ix.admit(".hidden", false) {
		t.Fatal("hidden files are admitted when the configuration includes them")
	}
}
