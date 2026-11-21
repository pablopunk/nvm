package emoji

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// The counts below are the TypeScript source's at generation time. A regenerated catalog that changes
// them should update this test in the same change, so data drift is a conscious decision.
// wantFlaggedNoVariant counts the skin-tone-flagged glyphs whose TypeScript lookup finds no variants
// (the person sequences with a ZWJ, such as 🕵️‍♂️); they keep their glyph in every tone.
const (
	wantEmojiCount       = 1914
	wantSymbolCount      = 1956
	wantCategoryCount    = 17
	wantFlaggedNoVariant = 8
)

func TestCatalogCountsMatchTheTypeScriptSource(t *testing.T) {
	var emoji, symbols int
	for _, entry := range NewCatalog().All() {
		switch entry.Kind() {
		case KindEmoji:
			emoji++
		case KindSymbol:
			symbols++
		}
	}
	if emoji != wantEmojiCount || symbols != wantSymbolCount {
		t.Errorf("catalog has %d emoji and %d symbols, want %d and %d", emoji, symbols, wantEmojiCount, wantSymbolCount)
	}
	if got := len(NewCatalog().Categories()); got != wantCategoryCount {
		t.Errorf("categories = %d, want %d", got, wantCategoryCount)
	}
}

func TestEveryEntryIsWellFormed(t *testing.T) {
	catalog := NewCatalog()
	categories := catalog.Categories()
	for index, entry := range catalog.All() {
		if !entry.Valid() {
			t.Fatalf("entry %d is not valid", index)
		}
		if glyph := entry.Glyph(); glyph == "" || !utf8.ValidString(glyph) {
			t.Fatalf("entry %d has glyph %q", index, glyph)
		}
		if name := entry.Name(); name == "" || !utf8.ValidString(name) {
			t.Fatalf("entry %q has name %q", entry.Glyph(), name)
		}
		if !slices.Contains(categories, entry.Category()) {
			t.Fatalf("entry %q has category %q outside Categories()", entry.Glyph(), entry.Category())
		}
		if kind := entry.Kind(); kind != KindEmoji && kind != KindSymbol {
			t.Fatalf("entry %q has kind %v", entry.Glyph(), kind)
		}
		if got, ok := catalog.Lookup(entry.ID()); !ok || got.Glyph() != entry.Glyph() || got.Kind() != entry.Kind() {
			t.Fatalf("Lookup(%q) does not return the entry", entry.ID())
		}
	}
}

func TestEntryIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range NewCatalog().All() {
		if seen[entry.ID()] {
			t.Fatalf("duplicate id %q", entry.ID())
		}
		seen[entry.ID()] = true
	}
}

func TestZeroEntryIsNotValid(t *testing.T) {
	var zero Entry
	if zero.Valid() || zero.Glyph() != "" || zero.Kind() != 0 || zero.SupportsSkinTone() {
		t.Error("zero Entry must be inert")
	}
}

func TestCategoriesAreDistinctAndEmojiFirst(t *testing.T) {
	categories := NewCatalog().Categories()
	if len(categories) == 0 || categories[0] != "Smileys & Emotion" {
		t.Fatalf("first category = %v, want Smileys & Emotion", categories)
	}
	if len(slices.Compact(slices.Sorted(slices.Values(categories)))) != len(categories) {
		t.Errorf("categories are not distinct: %v", categories)
	}
	if !slices.Contains(categories, "Currency") {
		t.Error("Currency category missing")
	}
}

func TestSkinVariantTableIsSortedAndWellFormed(t *testing.T) {
	for index := 1; index < len(skinVariants); index++ {
		if stringAt(skinVariants[index-1].base) >= stringAt(skinVariants[index].base) {
			t.Fatalf("skinVariants not strictly sorted at %d", index)
		}
	}
	for _, row := range skinVariants {
		base := stringAt(row.base)
		bareBase := strings.ReplaceAll(base, "️", "")
		for offset, id := range row.tones {
			variant := stringAt(id)
			modifier := rune(0x1F3FB + offset)
			if !strings.ContainsRune(variant, modifier) {
				t.Fatalf("variant %q of %q lacks U+%X", variant, base, modifier)
			}
			stripped := strings.Map(dropModifierOrSelector, variant)
			if stripped != bareBase {
				t.Fatalf("variant %q of %q differs from its base beyond the modifier", variant, base)
			}
		}
	}
}

func dropModifierOrSelector(r rune) rune {
	if r == '️' || (r >= 0x1F3FB && r <= 0x1F3FF) {
		return -1
	}
	return r
}

func TestFlaggedEmojiWithoutVariantKeysKeepTheirGlyph(t *testing.T) {
	var missing []string
	for _, entry := range NewCatalog().All() {
		if !entry.SupportsSkinTone() {
			continue
		}
		if _, ok := entry.SkinToneVariants(); !ok {
			missing = append(missing, entry.Glyph())
			if got := entry.WithSkinTone(SkinToneDark); got != entry.Glyph() {
				t.Errorf("%q changed to %q without variants", entry.Glyph(), got)
			}
		}
	}
	if len(missing) != wantFlaggedNoVariant {
		t.Errorf("%d flagged glyphs lack variant keys (%v), want %d", len(missing), missing, wantFlaggedNoVariant)
	}
}

func TestTableLayoutsStayInBounds(t *testing.T) {
	for index := 1; index < len(stringOffsets); index++ {
		if stringOffsets[index] < stringOffsets[index-1] {
			t.Fatalf("string offset %d decreases", index)
		}
	}
	if int(stringOffsets[len(stringOffsets)-1]) != len(catalogText) {
		t.Fatalf("last string offset %d, text length %d", stringOffsets[len(stringOffsets)-1], len(catalogText))
	}
	for index, r := range records {
		end := int(r.keywordStart) + int(r.keywordCount)
		if end > len(keywordIDs) {
			t.Fatalf("record %d keywords end at %d, past %d", index, end, len(keywordIDs))
		}
		if int(r.glyph) >= len(stringOffsets)-1 || int(r.name) >= len(stringOffsets)-1 || int(r.group) >= len(stringOffsets)-1 {
			t.Fatalf("record %d references a missing string", index)
		}
	}
}
