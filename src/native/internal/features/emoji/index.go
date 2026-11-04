package emoji

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// lookups are derived once from the static records: the lowercase Unicode label, group name and copy action title each
// search matches, the rune mask that skips records, the id map behind Lookup and Recents, and the single-character map
// behind UnicodeInfo.
type lookups struct {
	labels     []string
	groups     []string
	copyTitles []string
	masks      []charMask
	byID       map[string]uint16
	byRune     map[rune]uint16
}

// The titles 1.x gives a picker row's actions with frontmost paste available. 1.x's search matches them, so a query
// such as "unicode" matches every character, as it does there.
const (
	copyUnicodeTitle   = "copy unicode"
	pasteSkinToneTitle = "paste with skin tone"
)

var builtinLookups = sync.OnceValue(buildLookups)

func buildLookups() *lookups {
	idx := &lookups{
		labels:     make([]string, len(records)),
		groups:     make([]string, len(records)),
		copyTitles: make([]string, len(records)),
		masks:      make([]charMask, len(records)),
		byID:       make(map[string]uint16, len(records)),
		byRune:     make(map[rune]uint16, len(records)),
	}
	lowerGroups := map[uint16]string{}
	for index, rec := range records {
		entry := entryAt(index)
		codepoints := entry.Codepoints()
		idx.labels[index] = strings.ToLower(codepoints)
		idx.groups[index] = lowerGroup(lowerGroups, rec.group)
		idx.copyTitles[index] = "copy " + strings.ToLower(entry.Name())
		idx.masks[index].addText(entry.Glyph())
		for _, field := range entry.Keywords() {
			idx.masks[index].addText(strings.ToLower(field))
		}
		idx.masks[index].addText(idx.copyTitles[index])
		idx.masks[index].addText(copyUnicodeTitle)
		if rec.skinTone {
			idx.masks[index].addText(pasteSkinToneTitle)
		}
		idx.byID[entry.Kind().String()+":"+codepoints] = uint16(index)
		if r, ok := baseRune(entry.Glyph()); ok {
			if _, taken := idx.byRune[r]; !taken {
				idx.byRune[r] = uint16(index)
			}
		}
	}
	return idx
}

// lowerGroup returns the lowercase form of a group name, computing it once per group.
func lowerGroup(cache map[uint16]string, id uint16) string {
	if lower, ok := cache[id]; ok {
		return lower
	}
	lower := strings.ToLower(stringAt(id))
	cache[id] = lower
	return lower
}

// Prepare builds the search indexes now instead of on first use. The app calls it off the UI path at
// startup so the first keystroke does not pay for it.
func (c *Catalog) Prepare() {
	builtinLookups()
}

// baseRune returns the single character a glyph stands for once U+FE0F is removed, so "☝️" maps
// to U+261D. Multi-character sequences such as ZWJ families have no base rune.
func baseRune(glyph string) (rune, bool) {
	bare := strings.ReplaceAll(glyph, "️", "")
	r, size := utf8.DecodeRuneInString(bare)
	if size == 0 || size != len(bare) {
		return 0, false
	}
	return r, true
}
