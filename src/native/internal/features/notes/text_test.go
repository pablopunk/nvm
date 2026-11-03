package notes

import (
	"strings"
	"testing"
	"time"
)

// Ported from src/app/palette/extension-view/editor-title.test.ts: "derives note titles from the first non-empty
// Markdown line" (three assertions, one case each below).
func TestTitleDerivesFromFirstNonEmptyMarkdownLine(t *testing.T) {
	cases := []struct {
		name, markdown, want string
	}{
		{"heading after a blank line", "\n# A durable idea\nMore detail", "A durable idea"},
		{"empty content", "", "Untitled note"},
		{"emphasis around a link", "***[Formatted](https://example.com)***", "Formatted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustEqual(t, "Title", Title(tc.markdown), tc.want)
		})
	}
}

// The rules of editor-title.ts beyond its test: list, quote and checkbox markers, images, decoration, collapsed
// spaces, CRLF line endings, and the 120 character limit.
func TestTitleRulesOfTheEditorTitle(t *testing.T) {
	cases := []struct {
		name, markdown, want string
	}{
		{"list marker", "- buy milk", "buy milk"},
		{"ordered marker", "3. ship it", "ship it"},
		{"quote marker", "> quoted", "quoted"},
		{"checkbox", "[ ] pack bags", "pack bags"},
		{"checked box", "[x] done", "done"},
		{"image keeps alt text", "![diagram](a.png) plan", "diagram plan"},
		{"strike and code marks", "~~old~~ `new`", "old new"},
		{"collapses spaces", "a    b\t c", "a b c"},
		{"CRLF line ending", "first\r\nsecond", "first"},
		{"whitespace only", "   \n  ", "Untitled note"},
		{"blank lines before content", "\n\n   \nreal title", "real title"},
		{"only the first line", "one\ntwo", "one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustEqual(t, "Title", Title(tc.markdown), tc.want)
		})
	}
	long := strings.Repeat("é", 200)
	mustEqual(t, "long title rune count", len([]rune(Title(long))), titleLimit)
}

func TestPreviewReadsTheSecondContentLine(t *testing.T) {
	cases := []struct {
		name, markdown, want string
	}{
		{"second line", "# Title\n\nbody text", "body text"},
		{"one line", "# Title", "Markdown note"},
		{"empty", "  \n\n", "Empty note"},
		{"heading marks removed", "# T\n## Second heading", "Second heading"},
		{"limit", "t\n" + strings.Repeat("x", 150), strings.Repeat("x", previewLimit)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustEqual(t, "Preview", Preview(tc.markdown), tc.want)
		})
	}
}

func TestRelativeTimeUsesTheListWords(t *testing.T) {
	now := time.Date(2026, time.November, 3, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{0, "just now"},
		{59 * time.Second, "just now"},
		{time.Minute, "1m ago"},
		{59*time.Minute + 59*time.Second, "59m ago"},
		{time.Hour, "1h ago"},
		{23 * time.Hour, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{72 * time.Hour, "3d ago"},
		{-time.Minute, "just now"},
	}
	for _, tc := range cases {
		mustEqual(t, "RelativeTime", RelativeTime(now, now.Add(-tc.ago)), tc.want)
	}
}

func TestTruncateTextCutsOnARuneBoundary(t *testing.T) {
	text := strings.Repeat("a", MaxTextBytes-1) + "é"
	cut := truncateText(text)
	mustEqual(t, "length", len(cut), MaxTextBytes-1)
	mustEqual(t, "unchanged", truncateText("short"), "short")
}
