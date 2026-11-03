package emoji

import "testing"

func TestApplySkinToneUsesEveryVariant(t *testing.T) {
	want := []string{"👋🏻", "👋🏼", "👋🏽", "👋🏾", "👋🏿"}
	for index, glyph := range want {
		tone := SkinTone(index + 1)
		if got := ApplySkinTone("👋", tone); got != glyph {
			t.Errorf("ApplySkinTone(👋, %d) = %q, want %q", tone, got, glyph)
		}
	}
}

func TestApplySkinToneLeavesUnsupportedInputsAlone(t *testing.T) {
	tests := []struct {
		name  string
		glyph string
		tone  SkinTone
	}{
		{"default tone", "👋", SkinToneDefault},
		{"tone beyond dark", "👋", SkinTone(6)},
		{"glyph without variants", "🙂", SkinToneLight},
		{"symbol", "✓", SkinToneDark},
		{"empty", "", SkinToneMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplySkinTone(tt.glyph, tt.tone); got != tt.glyph {
				t.Errorf("ApplySkinTone(%q, %d) = %q, want the glyph unchanged", tt.glyph, tt.tone, got)
			}
		})
	}
}

func TestSkinToneVariantsOfReportsPresence(t *testing.T) {
	variants, ok := SkinToneVariantsOf("🖐️")
	if !ok || variants[4] != "🖐🏿" {
		t.Errorf("SkinToneVariantsOf(🖐️) = %v, %v; want the hand variants", variants, ok)
	}
	if _, ok := SkinToneVariantsOf("✓"); ok {
		t.Error("a symbol reported skin tone variants")
	}
}
