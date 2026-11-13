package search

import "testing"

// The vectors below are ported verbatim from 1.x/palette/search-ranking.test.ts (the scoreNormalized
// cases that depend on TypeScript normalization are left out: Score trims and lowercases on its own).
// Scores are for lowercase input, as ScoreFuzzy requires.

func TestScoreBandsMatchTheRankingContract(t *testing.T) {
	tests := []struct {
		name, text, query string
		want              int
	}{
		{"exact match", "hello", "hello", 100},
		{"single character exact", "a", "a", 100},
		{"startsWith at a word boundary", "hello-world", "hello", 90},
		{"startsWith inside a word", "abc", "a", 80},
		{"startsWith inside a word, longer", "nevermind", "never", 80},
		{"whole word after a space", "open terminal", "terminal", 90},
		{"whole word after a hyphen", "open-terminal", "terminal", 90},
		{"whole word before a space", "terminal settings", "terminal", 90},
		{"substring inside a word", "party", "art", 50},
		{"prefix of a longer word", "terminally", "terminal", 80},
		{"accented prefix", "café", "caf", 80},
		{"accented prefix, Spanish", "mañana", "ma", 80},
		{"CJK prefix", "東京タワー", "東京", 80},
		{"whole word at the end", "hello world", "world", 90},
		{"whole word after a space, settings", "open settings", "settings", 90},
		{"substring of digits", "abc123", "123", 50},
		{"fuzzy subsequence", "hello", "hlo", 20},
		{"fuzzy subsequence, typescript", "typescript", "ts", 20},
		{"fuzzy subsequence, settings", "settings", "stng", 20},
		{"word prefixes in order at the start", "rightwards arrow", "right arrow", 95},
		{"word prefixes in order, later", "move rightwards arrow", "right arrow", 70},
		{"word prefixes out of order", "rightwards arrow", "arrow right", 60},
		{"word prefixes in order inside a longer title", "fix selected text with ai", "fix with ai", 95},
		{"whole phrase after another word", "automate fix with ai", "fix with ai", 90},
		{"no match", "hello", "xyz", 0},
		{"no match, partial letters", "abc", "abx", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScoreFuzzy(tt.text, tt.query); got != tt.want {
				t.Fatalf("ScoreFuzzy(%q, %q) = %d, want %d", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestRepeatedWordPrefixMatchesAreConsumed(t *testing.T) {
	if got := ScoreFuzzy("go home", "go go"); got != 0 {
		t.Fatalf("ScoreFuzzy(go home, go go) = %d, want 0", got)
	}
	if got := ScoreFuzzy("go go home", "go go"); got != 95 {
		t.Fatalf("ScoreFuzzy(go go home, go go) = %d, want 95", got)
	}
}

func TestMixedCaseIsNotMatchedByScoreFuzzy(t *testing.T) {
	if got := ScoreFuzzy("Hello", "hello"); got != 0 {
		t.Fatalf("ScoreFuzzy(Hello, hello) = %d, want 0 (callers lowercase first)", got)
	}
	if got := Score("Hello", "hello"); got != 100 {
		t.Fatalf("Score(Hello, hello) = %d, want 100", got)
	}
}

func TestScoreTrimsAndLowercases(t *testing.T) {
	if got := Score("  Hello  ", "hello"); got != 100 {
		t.Fatalf("Score with surrounding space = %d, want 100", got)
	}
	if got := Score("HELLO WORLD", "world"); got != 90 {
		t.Fatalf("Score(HELLO WORLD, world) = %d, want 90", got)
	}
}

func TestCharacterSetFallbackScores10(t *testing.T) {
	tests := []struct {
		text, query string
		want        int
	}{
		{"gmail", "gmial", 10},
		{"typing", "tpying", 10},
		{"hello", "xyz", 0},
		{"abc", "abcd", 0},
		{"hello", "helloo", 0},
		{"hello", "helll", 0},
		{"hello", "heloo", 0},
	}
	for _, tt := range tests {
		if got := ScoreFuzzy(tt.text, tt.query); got != tt.want {
			t.Errorf("ScoreFuzzy(%q, %q) = %d, want %d", tt.text, tt.query, got, tt.want)
		}
	}
}

// TestEveryBandIsReachable names each documented band once: 100, 95, 90, 80, 70, 60, 50, 20 and 10.
func TestEveryBandIsReachable(t *testing.T) {
	seen := map[int]bool{}
	for _, c := range [][2]string{
		{"hello", "hello"}, {"rightwards arrow", "right arrow"}, {"hello world", "hello"}, {"abc", "a"},
		{"move rightwards arrow", "right arrow"}, {"rightwards arrow", "arrow right"}, {"party", "art"},
		{"hello", "hlo"}, {"gmail", "gmial"},
	} {
		seen[ScoreFuzzy(c[0], c[1])] = true
	}
	for _, band := range []int{100, 95, 90, 80, 70, 60, 50, 20, 10} {
		if !seen[band] {
			t.Errorf("band %d is not produced by any vector", band)
		}
	}
}

// urlVectors are the URL detection cases of search-utils.ts getUrlFromQuery.
var urlVectors = []struct{ query, want string }{
	{"github.com", "https://github.com"},
	{"open github.com", "https://github.com"},
	{"OPEN nevermind.app/docs", "https://nevermind.app/docs"},
	{"https://example.com/a?b=c", "https://example.com/a?b=c"},
	{"http://localhost.test", "http://localhost.test"},
	{"  github.com  ", "https://github.com"},
	{"a.b", "https://a.b"},
	{"nevermind.app:8080", "https://nevermind.app:8080"},
	{"localhost", ""},
	{"hello world", ""},
	{"foo bar.com", ""},
	{"", ""},
	{"open ", ""},
	{"2+2", ""},
	{"12 km to mi", ""},
}

func TestURLFromQueryDetectsWebsites(t *testing.T) {
	for _, v := range urlVectors {
		if got := URLFromQuery(v.query); got != v.want {
			t.Errorf("URLFromQuery(%q) = %q, want %q", v.query, got, v.want)
		}
	}
}
