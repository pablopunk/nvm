package emoji

import (
	"slices"
	"testing"
)

// These tests port 1.x/extensions/emoji-symbols.test.ts and 1.x/electron/app/emoji-symbol-catalog.test.ts.
// Grid, action, capability and view assertions are not ported: this package holds data and logic only,
// and the picker view that owns those shapes arrives in a later work package. Each such assertion is
// noted where it would sit.

func findGlyph(t *testing.T, glyph string) Entry {
	t.Helper()
	for _, entry := range NewCatalog().All() {
		if entry.Glyph() == glyph {
			return entry
		}
	}
	t.Fatalf("no catalog entry for %q", glyph)
	return Entry{}
}

// TS: "exposes a searchable grid with emoji and Unicode symbols".
// Ported: item count, grinning/infinity/check mark titles, keywords. Not ported: view type, columns,
// maxVisibleItems and subtitle (view shape); the subtitle assertion has no counterpart because
// entries carry no subtitle.
func TestTSExposesSearchableEmojiAndSymbols(t *testing.T) {
	catalog := NewCatalog()
	if got := len(catalog.All()); got <= 2000 {
		t.Fatalf("len(All()) = %d, want more than 2000", got)
	}
	if got := catalog.NormalizeCategory(""); got != AllCategory {
		t.Errorf("default category = %q, want %q", got, AllCategory)
	}
	grinning := findGlyph(t, "😀")
	if grinning.Title() != "Grinning Face" {
		t.Errorf("grinning title = %q, want %q", grinning.Title(), "Grinning Face")
	}
	if !slices.Contains(grinning.Keywords(), "smile") {
		t.Errorf("grinning keywords %v lack smile", grinning.Keywords())
	}
	infinity := findGlyph(t, "∞")
	if infinity.Title() != "Infinity" {
		t.Errorf("infinity title = %q, want Infinity", infinity.Title())
	}
	if !slices.Contains(infinity.Keywords(), "U+221E") {
		t.Errorf("infinity keywords %v lack U+221E", infinity.Keywords())
	}
	checkMark := findGlyph(t, "✓")
	if checkMark.Title() != "Check Mark" {
		t.Errorf("check mark title = %q, want Check Mark", checkMark.Title())
	}
}

// TS: "pastes by default and exposes copy, skin tone, and Unicode actions".
// Ported: title, the skin tone submenu titles and glyphs, the subtitle-free waving hand, the skin tone
// presence and the Copy Unicode text. Not ported: primary action, action order and mode (action shape,
// owned by the picker view).
func TestTSWavingHandSkinToneVariantsAndUnicodeCopy(t *testing.T) {
	waving := findGlyph(t, "👋")
	if waving.Title() != "Waving Hand" {
		t.Errorf("title = %q, want Waving Hand", waving.Title())
	}
	if !waving.SupportsSkinTone() {
		t.Fatal("waving hand must offer skin tones")
	}
	want := []string{
		"Default 👋",
		"Light 👋🏻",
		"Medium-light 👋🏼",
		"Medium 👋🏽",
		"Medium-dark 👋🏾",
		"Dark 👋🏿",
	}
	tones := []SkinTone{SkinToneDefault, SkinToneLight, SkinToneMediumLight, SkinToneMedium, SkinToneMediumDark, SkinToneDark}
	var got []string
	for _, tone := range tones {
		got = append(got, tone.Label()+" "+waving.WithSkinTone(tone))
	}
	if !slices.Equal(got, want) {
		t.Errorf("skin tone titles = %v, want %v", got, want)
	}
	if copied := CodepointsOf(waving.Glyph()); copied != "U+1F44B" {
		t.Errorf("Copy Unicode text = %q, want U+1F44B", copied)
	}
}

// TS: "falls back to copy when frontmost paste is unavailable".
// Ported: only the data fact the fallback relies on (the skin tone action exists for waving hand). The
// paste/copy choice and subtitles depend on system capabilities and are not ported.
func TestTSSkinToneActionDataForCopyFallback(t *testing.T) {
	if !findGlyph(t, "👋").SupportsSkinTone() {
		t.Error("waving hand lost its skin tone flag")
	}
}

// TS: "category changes persist and replace the current picker view".
// Ported: the Currency category returns more than ten entries, each in the category and tagged with it
// as a keyword. Not ported: persistence and view replacement (state and navigation).
func TestTSCategoryFilterReturnsOnlyThatCategory(t *testing.T) {
	results := NewCatalog().Search("", Options{Category: "Currency"})
	if len(results) <= 10 {
		t.Fatalf("Currency returned %d entries, want more than 10", len(results))
	}
	for _, entry := range results {
		if entry.Category() != "Currency" {
			t.Errorf("%q category = %q, want Currency", entry.Glyph(), entry.Category())
		}
		if !slices.Contains(entry.Keywords(), "Currency") {
			t.Errorf("%q keywords %v lack Currency", entry.Glyph(), entry.Keywords())
		}
	}
}

// TS: "skin tones precede variation selectors in canonical emoji sequences".
// Ported: the pointing index glyph with a variation selector offers Light as its first variant, with the
// tone index 1 the TypeScript action carried.
func TestTSSkinTonePrecedesVariationSelector(t *testing.T) {
	pointing := findGlyph(t, "☝️")
	variants, ok := pointing.SkinToneVariants()
	if !ok {
		t.Fatal("pointing index has no skin tone variants")
	}
	if variants[0] != "☝🏻" {
		t.Errorf("light variant = %q, want ☝🏻", variants[0])
	}
	if got := pointing.WithSkinTone(SkinToneLight); got != "☝🏻" {
		t.Errorf("WithSkinTone(Light) = %q, want ☝🏻", got)
	}
	if int(SkinToneLight) != 1 {
		t.Errorf("SkinToneLight = %d, want the TypeScript index 1", SkinToneLight)
	}
}

// TS: "catalog exposes stable opaque ids for emoji and symbols".
func TestTSCatalogExposesStableIDs(t *testing.T) {
	catalog := NewCatalog()
	grinning, ok := catalog.Lookup("emoji:U+1F600")
	if !ok || grinning.Glyph() != "😀" {
		t.Errorf("Lookup(emoji:U+1F600) = %q, %v; want 😀", grinning.Glyph(), ok)
	}
	check, ok := catalog.Lookup("symbol:U+2713")
	if !ok || check.Name() != "check mark" {
		t.Errorf("Lookup(symbol:U+2713) name = %q, %v; want check mark", check.Name(), ok)
	}
}

// TS: "formats Unicode code points for multi-codepoint sequences".
func TestTSFormatsCodepointsOfSequences(t *testing.T) {
	if got := CodepointsOf("👩‍💻"); got != "U+1F469 U+200D U+1F4BB" {
		t.Errorf("CodepointsOf = %q, want U+1F469 U+200D U+1F4BB", got)
	}
}

// TS: "applies skin tones after variation selectors and before joiners".
func TestTSAppliesSkinTonesBeforeJoiners(t *testing.T) {
	tests := []struct {
		glyph string
		tone  SkinTone
		want  string
	}{
		{"☝️", SkinToneLight, "☝🏻"},
		{"👩‍💻", SkinToneMedium, "👩🏽‍💻"},
		{"🧑‍⚕️", SkinToneMedium, "🧑🏽‍⚕️"},
		{"🧑‍🤝‍🧑", SkinToneMedium, "🧑🏽‍🤝‍🧑🏽"},
	}
	for _, tt := range tests {
		if got := ApplySkinTone(tt.glyph, tt.tone); got != tt.want {
			t.Errorf("ApplySkinTone(%q, %d) = %q, want %q", tt.glyph, tt.tone, got, tt.want)
		}
	}
}
