package emoji

import (
	"slices"
	"strings"
)

// Options narrows a search. The zero value searches every category without a limit.
type Options struct {
	Category string // a group name from Categories; AllCategory, "" or an unknown name searches all
	Limit    int    // at most this many results; zero or negative means no limit
}

// The ranks of a matched entry, as 1.x's grid orders them: the entry whose name is the query, then the entries with a
// keyword equal to it, then the rest.
const (
	rankTitle = iota
	rankKeyword
	rankPartial
)

// nameField is the position of the lowercase name among an entry's identity fields.
const nameField = 1

type rankedRecord struct {
	index int
	rank  int
}

// Search returns the entries matching query in 1.x's order: the entry named by the query first, then the entries with
// a keyword equal to it, then the rest, each group in catalog order. An empty query lists the category in catalog order
// without matching.
func (c *Catalog) Search(query string, opts Options) []Entry {
	needle := newSearchQuery(normalizeQuery(query))
	category := c.NormalizeCategory(opts.Category)
	categoryID, filtered := categoryIDOf(category)
	idx := builtinLookups()
	var matches []rankedRecord
	var fields []string
	for index, r := range records {
		if filtered && r.group != categoryID {
			continue
		}
		if needle.text == "" {
			matches = append(matches, rankedRecord{index: index})
			continue
		}
		if !idx.masks[index].covers(needle.mask) {
			continue
		}
		var identity int
		fields, identity = recordFields(fields[:0], r, idx, index)
		rank, matched := rankRecord(fields, identity, needle)
		if matched {
			matches = append(matches, rankedRecord{index: index, rank: rank})
		}
	}
	if needle.text != "" {
		slices.SortStableFunc(matches, compareRankedRecords)
	}
	return entriesOf(matches, opts.Limit)
}

func compareRankedRecords(a, b rankedRecord) int {
	return a.rank - b.rank
}

func entriesOf(matches []rankedRecord, limit int) []Entry {
	count := len(matches)
	if limit > 0 {
		count = min(count, limit)
	}
	out := make([]Entry, 0, count)
	for _, match := range matches[:count] {
		out = append(out, entryAt(match.index))
	}
	return out
}

// recordFields appends the searchable values of one record, lowercased where the matcher folds case: the identity
// fields (glyph, name, group, Unicode values, keywords), then the titles 1.x gives the record's actions, which its search
// matches too. It returns the fields and how many of them are identity fields.
func recordFields(dst []string, r record, idx *lookups, index int) ([]string, int) {
	dst = append(dst, stringAt(r.glyph), strings.ToLower(stringAt(r.name)), idx.groups[index], idx.labels[index])
	for offset := range int(r.keywordCount) {
		dst = append(dst, strings.ToLower(stringAt(keywordIDs[int(r.keywordStart)+offset])))
	}
	identity := len(dst)
	dst = append(dst, idx.copyTitles[index], copyUnicodeTitle)
	if r.skinTone {
		dst = append(dst, pasteSkinToneTitle)
	}
	return dst, identity
}

// rankRecord reports whether a record matches through any of its fields, and the rank of its identity fields.
func rankRecord(fields []string, identity int, q searchQuery) (int, bool) {
	for _, field := range fields {
		if matchesQuery(field, q) {
			return rankOf(fields[:identity], q), true
		}
	}
	return 0, false
}

// rankOf returns the rank of a record by its identity fields: rankTitle when the name is the query, rankKeyword when
// another identity field is, and rankPartial otherwise.
func rankOf(identity []string, q searchQuery) int {
	rank := rankPartial
	for position, field := range identity {
		if field != q.text {
			continue
		}
		if position == nameField {
			return rankTitle
		}
		rank = rankKeyword
	}
	return rank
}
