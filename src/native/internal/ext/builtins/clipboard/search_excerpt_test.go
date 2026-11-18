package clipboard

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSearchExcerptBoundsLongQueryAndPreservesUnicode(t *testing.T) {
	query := strings.Repeat("a", 4096) + "b"
	text := strings.Repeat("界", 30) + query + " after"
	got := matchExcerpt(text, "unrelated", query)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) > excerptRunes+2 || !strings.HasSuffix(got, "…") {
		t.Fatalf("excerpt must be valid and bounded: %d runes", utf8.RuneCountInString(got))
	}
}

func TestSearchExcerptLocatesMatchAfterCaseMappingChangesByteWidth(t *testing.T) {
	got := matchExcerpt(strings.Repeat("İ", 40)+" PULSE target", "unrelated", "pulse")
	if !strings.Contains(got, "PULSE target") {
		t.Fatalf("case mapping moved excerpt away from match: %q", got)
	}
}

func TestSearchExcerptRejectsLongOverlappingNonmatch(t *testing.T) {
	if got := matchExcerpt(strings.Repeat("a", 65536), "unrelated", strings.Repeat("a", 4096)+"b"); got != "" {
		t.Fatal("nonmatch produced an excerpt")
	}
}
