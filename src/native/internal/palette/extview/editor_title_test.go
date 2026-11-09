package extview

import (
	"strings"
	"testing"
)

func TestTitleFromFirstLine(t *testing.T) {
	tests := map[string]string{
		"# Heading\nbody":             "Heading",
		"\n\n  plain line":            "plain line",
		"- [x] done item":             "done item",
		"**bold** and [link](http://a)": "bold and link",
		"![alt](pic.png) text":        "alt text",
		"":                            "Fallback",
	}
	for content, want := range tests {
		if got := titleFromFirstLine(content, "Fallback"); got != want {
			t.Errorf("%q = %q, want %q", content, got, want)
		}
	}
	if got := titleFromFirstLine(strings.Repeat("é", 200), "x"); len([]rune(got)) != maxEditorTitleRunes {
		t.Errorf("long title kept %d runes", len([]rune(got)))
	}
}
