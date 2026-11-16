package search

import "testing"

func TestTierOfTitle(t *testing.T) {
	tests := []struct {
		name, title, query string
		aliases, keywords  []string
		want               Tier
	}{
		{"exact", "Loom", "loom", nil, nil, TierExact},
		{"exact ignores case and accents", "Café Olé", "CAFE OLE", nil, nil, TierExact},
		{"exact normalizes spaces", "Google  Chrome", "  google chrome ", nil, nil, TierExact},
		{"exact alias", "Google Chrome", "gc", []string{"GC"}, nil, TierExact},
		{"exact keyword", "Check for Updates", "release", nil, []string{"release"}, TierExact},
		{"alias that only contains the query is content", "Triage Selected Text", "cl", []string{"classify selected text"}, nil, TierContent},
		{"prefix", "Clipboard History", "clip", nil, nil, TierPrefix},
		{"prefix with a space", "Log out of Nevermind", "log o", nil, nil, TierPrefix},
		{"word start", "Open Settings", "sett", nil, nil, TierWordStart},
		{"word start after a hyphen", "cloud-setup.jpg", "set", nil, nil, TierWordStart},
		{"initials", "Google Chrome", "gc", nil, nil, TierWordStart},
		{"initials of a longer title", "Visual Studio Code", "vsc", nil, nil, TierWordStart},
		{"query words start successive words", "Fix Selected Text with AI", "fix with ai", nil, nil, TierWordStart},
		{"substring", "Terminal", "mina", nil, nil, TierSubstring},
		{"scattered from a word start", "Log out of Nevermind", "loom", nil, nil, TierScattered},
		{"scattered needs two letters", "Terminal", "z", nil, nil, TierContent},
		{"scattered needs a word start", "Bottom Half", "loom", nil, nil, TierContent},
		{"scattered must start a word", "Settings", "es", nil, nil, TierContent},
		{"no match is content", "Terminal", "xyz", nil, nil, TierContent},
		{"a bag of letters is no match", "Gmail", "gmial", nil, nil, TierContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newQueryMatcher(tt.query)
			if got := m.tierOf(tt.title, append(tt.aliases, tt.keywords...)); got != tt.want {
				t.Fatalf("tier of %q for %q = %d, want %d", tt.title, tt.query, got, tt.want)
			}
		})
	}
}

func TestBlankQueryHasNoMatcher(t *testing.T) {
	if newQueryMatcher("  \t") != nil {
		t.Fatal("a blank query has a matcher")
	}
}

func TestMatchingAllocatesNothingForAnASCIITitle(t *testing.T) {
	m := newQueryMatcher("gc")
	m.tierOf("Google Chrome Canary", []string{"chrome"}) // sizes the scratch buffers
	allocs := testing.AllocsPerRun(100, func() { m.tierOf("Google Chrome Canary", []string{"chrome", "browser"}) })
	if allocs != 0 {
		t.Fatalf("matching allocates %.0f times per row, want none", allocs)
	}
}

func TestScoreNoLongerMatchesLooseLetters(t *testing.T) {
	tests := []struct {
		text, query string
		want        int
	}{
		{"gmail", "gmial", 0},
		{"Bottom Half", "loom", 0},
		{"Log out of Nevermind", "loom", 20},
		{"hello", "hlo", 20},
		{"hello", "elo", 0},
		{"typescript", "ts", 20},
		{"a", "a", 100},
	}
	for _, tt := range tests {
		if got := Score(tt.text, tt.query); got != tt.want {
			t.Errorf("Score(%q, %q) = %d, want %d", tt.text, tt.query, got, tt.want)
		}
	}
}
