package emoji

import "testing"

func TestUnicodeInfoForCatalogCharacter(t *testing.T) {
	info, ok := NewCatalog().UnicodeInfo('✓')
	if !ok {
		t.Fatal("UnicodeInfo rejected a valid rune")
	}
	want := UnicodeDetails{
		Glyph:     "✓",
		Codepoint: "U+2713",
		Decimal:   10003,
		UTF8:      "E2 9C 93",
		Name:      "check mark",
		Kind:      KindSymbol,
	}
	if info != want {
		t.Errorf("UnicodeInfo('✓') = %+v, want %+v", info, want)
	}
}

func TestUnicodeInfoNamesBaseCharacterOfVariationSequence(t *testing.T) {
	info, ok := NewCatalog().UnicodeInfo(0x261D)
	if !ok || info.Name != "index pointing up" || info.Kind != KindEmoji {
		t.Errorf("UnicodeInfo(U+261D) = %+v, %v; want the emoji index pointing up", info, ok)
	}
}

func TestUnicodeInfoForUncatalogedCharacter(t *testing.T) {
	info, ok := NewCatalog().UnicodeInfo('A')
	if !ok {
		t.Fatal("UnicodeInfo rejected A")
	}
	if info.Name != "" || info.Kind != KindUnicode || info.UTF8 != "41" || info.Codepoint != "U+0041" {
		t.Errorf("UnicodeInfo('A') = %+v", info)
	}
}

func TestUnicodeInfoRejectsNonScalarValues(t *testing.T) {
	for _, r := range []rune{0xD800, 0xDFFF, 0x110000, -1} {
		if _, ok := NewCatalog().UnicodeInfo(r); ok {
			t.Errorf("UnicodeInfo(%#x) accepted a non-scalar value", r)
		}
	}
}

func TestCodepointsOfKeepsJoinersAndSelectorsVisible(t *testing.T) {
	tests := map[string]string{
		"":           "",
		"A":          "U+0041",
		"é":          "U+00E9",
		"\U0010FFFF": "U+10FFFF",
		"☝️":         "U+261D U+FE0F",
		"🇺🇸":         "U+1F1FA U+1F1F8",
		"🏴\U000e0067\U000e0062\U000e0077\U000e006c\U000e0073\U000e007f": "U+1F3F4 U+E0067 U+E0062 U+E0077 U+E006C U+E0073 U+E007F",
	}
	for glyph, want := range tests {
		if got := CodepointsOf(glyph); got != want {
			t.Errorf("CodepointsOf(%q) = %q, want %q", glyph, got, want)
		}
	}
}

func TestCodepointsOfReplacesInvalidUTF8(t *testing.T) {
	if got := CodepointsOf("\xff"); got != "U+FFFD" {
		t.Errorf("CodepointsOf(invalid byte) = %q, want U+FFFD", got)
	}
}

func TestNormalizeQueryRewritesHexForms(t *testing.T) {
	tests := map[string]string{
		"  U+1F600 ": "u+1f600",
		"0x1F600":    "u+1f600",
		"0X1f600":    "u+1f600",
		"1f600":      "1f600",
		"0x":         "0x",
		"0xzz":       "0xzz",
		"0x1234567":  "0x1234567",
		"Grinning":   "grinning",
	}
	for query, want := range tests {
		if got := normalizeQuery(query); got != want {
			t.Errorf("normalizeQuery(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestTitleCaseCapitalizesASCIIWordsOnly(t *testing.T) {
	tests := map[string]string{
		"grinning face": "Grinning Face",
		"left-pointing double angle quotation mark": "Left-Pointing Double Angle Quotation Mark",
		"one o’clock": "One O’Clock",
		"x2 y_z":      "X2 Y_z",
		// Matches the TypeScript /\b\w/g: a non-ASCII byte before a letter counts as a boundary.
		"élan": "éLan",
	}
	for input, want := range tests {
		if got := titleCase(input); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSkinToneLabelsAndOutOfRangeTones(t *testing.T) {
	labels := []string{"Default", "Light", "Medium-light", "Medium", "Medium-dark", "Dark"}
	for index, want := range labels {
		if got := SkinTone(index).Label(); got != want {
			t.Errorf("SkinTone(%d).Label() = %q, want %q", index, got, want)
		}
	}
	if got := SkinTone(6).Label(); got != "" {
		t.Errorf("out of range label = %q, want empty", got)
	}
}
