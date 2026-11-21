package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// skinToneModifierBase is U+1F3FB, the first of the five Fitzpatrick modifiers (light to dark).
const skinToneModifierBase = 0x1F3FB

// source is the decoded TypeScript catalog, in source order.
type source struct {
	emoji   []entrySource
	symbols []entrySource
	skin    []skinSource
}

// entrySource is one EMOJI_RECORDS or SYMBOL_RECORDS row. Symbols have no keywords or skin tone.
type entrySource struct {
	glyph, name, group string
	keywords           []string
	skinTone           bool
	kindName           string
}

// skinSource is one SKIN_TONE_VARIANTS pair: the base glyph and its five modified glyphs.
type skinSource struct {
	base  string
	tones [5]string
}

// decodeSource reads the three exported declarations from the TypeScript data module.
func decodeSource(data []byte) (source, error) {
	text := string(data)
	emojiRows, err := parseExportedArray(text, "EMOJI_RECORDS")
	if err != nil {
		return source{}, err
	}
	symbolRows, err := parseExportedArray(text, "SYMBOL_RECORDS")
	if err != nil {
		return source{}, err
	}
	skinRows, err := parseExportedArray(text, "SKIN_TONE_VARIANTS")
	if err != nil {
		return source{}, err
	}
	var decoded source
	if decoded.emoji, err = decodeEntries(emojiRows, "EMOJI_RECORDS", "KindEmoji", true); err != nil {
		return source{}, err
	}
	if decoded.symbols, err = decodeEntries(symbolRows, "SYMBOL_RECORDS", "KindSymbol", false); err != nil {
		return source{}, err
	}
	if decoded.skin, err = decodeSkin(skinRows); err != nil {
		return source{}, err
	}
	return decoded, nil
}

func decodeEntries(rows []any, name, kindName string, emojiShape bool) ([]entrySource, error) {
	width := 3
	if emojiShape {
		width = 5
	}
	entries := make([]entrySource, 0, len(rows))
	for index, raw := range rows {
		fields, ok := raw.([]any)
		if !ok || len(fields) != width {
			return nil, fmt.Errorf("%s[%d]: want %d fields", name, index, width)
		}
		entry := entrySource{kindName: kindName}
		var err error
		if entry.glyph, err = stringField(fields[0], name, index, "glyph"); err != nil {
			return nil, err
		}
		if entry.name, err = stringField(fields[1], name, index, "name"); err != nil {
			return nil, err
		}
		if entry.group, err = stringField(fields[2], name, index, "group"); err != nil {
			return nil, err
		}
		if emojiShape {
			if entry.keywords, err = stringList(fields[3], name, index); err != nil {
				return nil, err
			}
			if entry.skinTone, err = skinToneFlag(fields[4], name, index); err != nil {
				return nil, err
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeSkin(rows []any) ([]skinSource, error) {
	skins := make([]skinSource, 0, len(rows))
	for index, raw := range rows {
		pair, ok := raw.([]any)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("SKIN_TONE_VARIANTS[%d]: want [base, variants]", index)
		}
		var skin skinSource
		var err error
		if skin.base, err = stringField(pair[0], "SKIN_TONE_VARIANTS", index, "base"); err != nil {
			return nil, err
		}
		variants, err := stringList(pair[1], "SKIN_TONE_VARIANTS", index)
		if err != nil {
			return nil, err
		}
		if len(variants) != len(skin.tones) {
			return nil, fmt.Errorf("SKIN_TONE_VARIANTS[%d]: want %d variants, got %d", index, len(skin.tones), len(variants))
		}
		copy(skin.tones[:], variants)
		skins = append(skins, skin)
	}
	return skins, nil
}

func stringField(value any, table string, index int, field string) (string, error) {
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("%s[%d].%s: want a non-empty string", table, index, field)
	}
	return text, nil
}

func stringList(value any, table string, index int) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s[%d]: want an array of strings", table, index)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, fmt.Errorf("%s[%d]: keywords must be non-empty strings", table, index)
		}
		out = append(out, text)
	}
	return out, nil
}

func skinToneFlag(value any, table string, index int) (bool, error) {
	flag, ok := value.(int)
	if !ok || (flag != 0 && flag != 1) {
		return false, fmt.Errorf("%s[%d]: supportsSkinTone must be 0 or 1", table, index)
	}
	return flag == 1, nil
}

// validateSource rejects data the Go tables cannot represent faithfully: duplicate entries,
// invalid UTF-8, and skin tone variants that do not carry their modifier.
func validateSource(decoded source) error {
	seen := map[string]bool{}
	for _, entry := range append(append([]entrySource{}, decoded.emoji...), decoded.symbols...) {
		if !utf8.ValidString(entry.glyph) || !utf8.ValidString(entry.name) {
			return fmt.Errorf("entry %q has invalid UTF-8", entry.name)
		}
		key := entry.kindName + "|" + entry.glyph
		if seen[key] {
			return fmt.Errorf("duplicate %s entry %q", entry.kindName, entry.glyph)
		}
		seen[key] = true
	}
	bases := map[string]bool{}
	for _, skin := range decoded.skin {
		if bases[skin.base] {
			return fmt.Errorf("duplicate skin tone base %q", skin.base)
		}
		bases[skin.base] = true
		for offset, variant := range skin.tones {
			modifier := rune(skinToneModifierBase + offset)
			if !strings.ContainsRune(variant, modifier) {
				return fmt.Errorf("skin tone variant %q for %q lacks modifier U+%X", variant, skin.base, modifier)
			}
		}
	}
	return nil
}
