package emoji

// Kind says what an entry is. The zero value is not a valid kind.
type Kind uint8

const (
	KindEmoji Kind = iota + 1
	KindSymbol
	KindUnicode
)

// String returns the lowercase kind name used in entry ids: "emoji", "symbol" or "unicode".
func (k Kind) String() string {
	switch k {
	case KindEmoji:
		return "emoji"
	case KindSymbol:
		return "symbol"
	case KindUnicode:
		return "unicode"
	default:
		return ""
	}
}

// record is one catalog row as laid out in catalog_data.go. Text fields are string ids into
// catalogText. The keywords are keywordIDs[keywordStart : keywordStart+keywordCount].
type record struct {
	glyph, name, group uint16
	keywordStart       uint16
	keywordCount       uint8
	kind               Kind
	skinTone           bool
}

// skinVariant maps a base glyph to its five skin tone glyphs, Light through Dark.
type skinVariant struct {
	base  uint16
	tones [5]uint16
}

// Entry is a handle to one catalog entry. The zero Entry is not valid: check Valid before use.
// Entries are cheap values; the strings they return point into the static catalog text.
type Entry struct {
	ref uint16 // record index plus one, so the zero value means no entry
}

func entryAt(index int) Entry {
	return Entry{ref: uint16(index + 1)}
}

// Valid reports whether the entry refers to a catalog record.
func (e Entry) Valid() bool {
	return e.ref != 0 && int(e.ref) <= len(records)
}

func (e Entry) record() record {
	if !e.Valid() {
		return record{}
	}
	return records[e.ref-1]
}

// Glyph returns the character or sequence as UTF-8, for example "👩‍💻" or "∞".
func (e Entry) Glyph() string {
	if !e.Valid() {
		return ""
	}
	return stringAt(e.record().glyph)
}

// Name returns the catalog name in lowercase, for example "grinning face".
func (e Entry) Name() string {
	if !e.Valid() {
		return ""
	}
	return stringAt(e.record().name)
}

// Title returns the name with each ASCII word capitalized, for display: "Grinning Face".
func (e Entry) Title() string {
	return titleCase(e.Name())
}

// Category returns the group name the entry belongs to, for example "Smileys & Emotion".
func (e Entry) Category() string {
	if !e.Valid() {
		return ""
	}
	return stringAt(e.record().group)
}

// Kind returns KindEmoji or KindSymbol.
func (e Entry) Kind() Kind {
	return e.record().kind
}

// Codepoints returns the Unicode values of the glyph, for example "U+1F469 U+200D U+1F4BB".
func (e Entry) Codepoints() string {
	return CodepointsOf(e.Glyph())
}

// ID returns the stable opaque identifier used by recents and actions, for example
// "emoji:U+1F600" or "symbol:U+2713".
func (e Entry) ID() string {
	if !e.Valid() {
		return ""
	}
	return e.Kind().String() + ":" + e.Codepoints()
}

// Keywords returns the searchable terms in the order the TypeScript picker listed them: the glyph,
// the name, the group, the Unicode values, then the curated keywords.
func (e Entry) Keywords() []string {
	r := e.record()
	out := make([]string, 0, 4+int(r.keywordCount))
	out = append(out, e.Glyph(), e.Name(), e.Category(), e.Codepoints())
	for index := range int(r.keywordCount) {
		out = append(out, stringAt(keywordIDs[int(r.keywordStart)+index]))
	}
	return out
}

// SupportsSkinTone reports whether the entry offers skin tone variants.
func (e Entry) SupportsSkinTone() bool {
	return e.record().skinTone
}

// SkinToneVariants returns the Light through Dark variants, and false when the glyph has none.
func (e Entry) SkinToneVariants() ([5]string, bool) {
	return SkinToneVariantsOf(e.Glyph())
}

// WithSkinTone returns the glyph in the requested skin tone; the default tone returns the glyph.
func (e Entry) WithSkinTone(tone SkinTone) string {
	return ApplySkinTone(e.Glyph(), tone)
}

func stringAt(id uint16) string {
	return catalogText[stringOffsets[id]:stringOffsets[id+1]]
}

// titleCase capitalizes the first ASCII letter of each word, matching the TypeScript
// replace(/\b\w/g) it replaces: word characters are ASCII only, so non-ASCII bytes separate words.
func titleCase(value string) string {
	out := []byte(value)
	for index, c := range out {
		if isASCIILower(c) && (index == 0 || !isASCIIWord(out[index-1])) {
			out[index] = c - 'a' + 'A'
		}
	}
	return string(out)
}

func isASCIILower(c byte) bool {
	return c >= 'a' && c <= 'z'
}

func isASCIIWord(c byte) bool {
	return isASCIILower(c) || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
