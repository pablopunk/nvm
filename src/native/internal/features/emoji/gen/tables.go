package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// catalogTables is the compact layout written to catalog_data.go. Text fields are ids into
// strings; keywords are keywordIDs[keywordStart : keywordStart+keywordCount].
type catalogTables struct {
	strings    []string
	categories []uint16
	keywordIDs []uint16
	records    []recordRow
	skins      []skinRow
}

type recordRow struct {
	glyph, name, group uint16
	keywordStart       int
	keywordCount       int
	kindName           string
	skinTone           bool
}

type skinRow struct {
	base  uint16
	tones [5]uint16
}

// stringTable interns strings in first-seen order, so the output depends only on the input order.
type stringTable struct {
	index  map[string]uint16
	values []string
}

func newStringTable() *stringTable {
	return &stringTable{index: map[string]uint16{}}
}

func (t *stringTable) id(value string) (uint16, error) {
	if id, ok := t.index[value]; ok {
		return id, nil
	}
	if len(t.values) >= math.MaxUint16 {
		return 0, fmt.Errorf("more than %d distinct strings", math.MaxUint16)
	}
	id := uint16(len(t.values))
	t.values = append(t.values, value)
	t.index[value] = id
	return id, nil
}

// buildTables lays the decoded source out in catalog order: emoji first, then symbols, as the
// TypeScript CHARACTER_RECORDS does. Skin tone rows are sorted by base glyph for binary search.
func buildTables(decoded source) (catalogTables, error) {
	strs := newStringTable()
	var out catalogTables
	seenGroup := map[uint16]bool{}
	entries := append(append([]entrySource{}, decoded.emoji...), decoded.symbols...)
	for _, entry := range entries {
		row, err := internRecord(strs, &out, entry)
		if err != nil {
			return catalogTables{}, err
		}
		if !seenGroup[row.group] {
			seenGroup[row.group] = true
			out.categories = append(out.categories, row.group)
		}
		out.records = append(out.records, row)
	}
	skins := slices.Clone(decoded.skin)
	slices.SortFunc(skins, compareSkinBases)
	for _, skin := range skins {
		row, err := internSkin(strs, skin)
		if err != nil {
			return catalogTables{}, err
		}
		out.skins = append(out.skins, row)
	}
	out.strings = strs.values
	return out, nil
}

func internRecord(strs *stringTable, out *catalogTables, entry entrySource) (recordRow, error) {
	row := recordRow{kindName: entry.kindName, skinTone: entry.skinTone}
	var err error
	if row.glyph, err = strs.id(entry.glyph); err != nil {
		return recordRow{}, err
	}
	if row.name, err = strs.id(entry.name); err != nil {
		return recordRow{}, err
	}
	if row.group, err = strs.id(entry.group); err != nil {
		return recordRow{}, err
	}
	if len(entry.keywords) > math.MaxUint8 {
		return recordRow{}, fmt.Errorf("%q has %d keywords; the layout allows %d", entry.glyph, len(entry.keywords), math.MaxUint8)
	}
	row.keywordStart = len(out.keywordIDs)
	row.keywordCount = len(entry.keywords)
	for _, keyword := range entry.keywords {
		id, err := strs.id(keyword)
		if err != nil {
			return recordRow{}, err
		}
		out.keywordIDs = append(out.keywordIDs, id)
	}
	if len(out.keywordIDs) > math.MaxUint16 {
		return recordRow{}, fmt.Errorf("more than %d keyword references", math.MaxUint16)
	}
	return row, nil
}

func internSkin(strs *stringTable, skin skinSource) (skinRow, error) {
	base, err := strs.id(skin.base)
	if err != nil {
		return skinRow{}, err
	}
	row := skinRow{base: base}
	for index, tone := range skin.tones {
		if row.tones[index], err = strs.id(tone); err != nil {
			return skinRow{}, err
		}
	}
	return row, nil
}

func compareSkinBases(a, b skinSource) int {
	return strings.Compare(a.base, b.base)
}
