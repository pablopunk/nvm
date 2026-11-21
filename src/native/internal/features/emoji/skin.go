package emoji

import (
	"slices"
	"strings"
)

// SkinTone is a Fitzpatrick skin tone. The default tone is the unmodified glyph.
type SkinTone uint8

const (
	SkinToneDefault SkinTone = iota
	SkinToneLight
	SkinToneMediumLight
	SkinToneMedium
	SkinToneMediumDark
	SkinToneDark
)

var skinToneLabels = [...]string{"Default", "Light", "Medium-light", "Medium", "Medium-dark", "Dark"}

// Label returns the picker label for the tone, for example "Medium-light". Unknown tones return "".
func (t SkinTone) Label() string {
	if int(t) >= len(skinToneLabels) {
		return ""
	}
	return skinToneLabels[t]
}

// ApplySkinTone returns glyph in the given skin tone. It mirrors the TypeScript characterWithSkinTone:
// the default tone, an unknown tone, or a glyph without variants returns the glyph unchanged. The
// lookup tries the glyph as written, then without U+FE0F, so "☝️" finds the "☝" variants.
func ApplySkinTone(glyph string, tone SkinTone) string {
	if tone == SkinToneDefault || tone > SkinToneDark {
		return glyph
	}
	tones, ok := lookupSkinTones(glyph)
	if !ok {
		return glyph
	}
	return stringAt(tones[tone-1])
}

// SkinToneVariantsOf returns the Light through Dark glyphs for glyph, and false when it has none.
func SkinToneVariantsOf(glyph string) ([5]string, bool) {
	tones, ok := lookupSkinTones(glyph)
	if !ok {
		return [5]string{}, false
	}
	var out [5]string
	for index, id := range tones {
		out[index] = stringAt(id)
	}
	return out, true
}

func lookupSkinTones(glyph string) ([5]uint16, bool) {
	if tones, ok := findSkinVariant(glyph); ok {
		return tones, true
	}
	if strings.ContainsRune(glyph, '️') {
		return findSkinVariant(strings.ReplaceAll(glyph, "️", ""))
	}
	return [5]uint16{}, false
}

func findSkinVariant(base string) ([5]uint16, bool) {
	index, found := slices.BinarySearchFunc(skinVariants[:], base, compareSkinBase)
	if !found {
		return [5]uint16{}, false
	}
	return skinVariants[index].tones, true
}

func compareSkinBase(row skinVariant, base string) int {
	return strings.Compare(stringAt(row.base), base)
}
