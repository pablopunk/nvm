package emoji

// charMask is a 128-bit set of runes: rune r sets bit r mod 128. Every band of the scorer needs each
// query rune to occur in some field, so a record whose mask lacks a query bit cannot match and is
// skipped without scoring its fields. ASCII runes map to themselves, so ASCII queries are exact;
// other runes may share a bit, which only admits extra records to scoring and never drops a match.
type charMask [2]uint64

func runeBit(r rune) uint {
	return uint(r) % 128
}

func (m *charMask) addText(text string) {
	for _, r := range text {
		bit := runeBit(r)
		m[bit>>6] |= uint64(1) << (bit & 63)
	}
}

func (m charMask) covers(query charMask) bool {
	return m[0]&query[0] == query[0] && m[1]&query[1] == query[1]
}

func maskOf(text string) charMask {
	var m charMask
	m.addText(text)
	return m
}
