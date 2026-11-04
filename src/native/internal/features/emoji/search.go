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

type scoredRecord struct {
	index int
	score int
	// named is set when the entry's name is the query itself, as "fire" is for the fire character.
	named bool
}

// Search returns the entries matching query, best first. Each field (glyph, name, group, Unicode
// values, keywords) scores with the shared bands and an entry takes its best field. Ties go to an entry
// whose name is the query, then keep catalog order. An empty query lists the category in catalog order
// without scoring.
func (c *Catalog) Search(query string, opts Options) []Entry {
	needle := newSearchQuery(normalizeQuery(query))
	category := c.NormalizeCategory(opts.Category)
	categoryID, filtered := categoryIDOf(category)
	idx := builtinLookups()
	var matches []scoredRecord
	var fields []string
	for index, r := range records {
		if filtered && r.group != categoryID {
			continue
		}
		score := 1
		named := false
		if needle.text != "" {
			if !idx.masks[index].covers(needle.mask) {
				continue
			}
			fields = recordFields(fields[:0], r, idx, index)
			score = recordScore(fields, needle)
			if score == 0 {
				continue
			}
			named = fields[1] == needle.text
		}
		matches = append(matches, scoredRecord{index: index, score: score, named: named})
	}
	if needle.text != "" {
		slices.SortStableFunc(matches, compareScoredRecords)
	}
	return entriesOf(matches, opts.Limit)
}

// compareScoredRecords orders by score, best first. Among equal scores an entry named by the query comes first, so
// "fire" lists the fire character before a firefighter, which matches only through a keyword.
func compareScoredRecords(a, b scoredRecord) int {
	if a.score != b.score {
		return b.score - a.score
	}
	return boolRank(b.named) - boolRank(a.named)
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}

func entriesOf(matches []scoredRecord, limit int) []Entry {
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

// recordFields appends the searchable values of one record, lowercased where the scorer folds case.
// The glyph is matched as written; the Unicode value and group forms are precomputed.
func recordFields(dst []string, r record, idx *lookups, index int) []string {
	dst = append(dst, stringAt(r.glyph), strings.ToLower(stringAt(r.name)), idx.groups[index], idx.labels[index])
	for offset := range int(r.keywordCount) {
		dst = append(dst, strings.ToLower(stringAt(keywordIDs[int(r.keywordStart)+offset])))
	}
	return dst
}

// recordScore returns the best ordered band over the fields. A query that matches no field in order matches nothing:
// scattered letters are not a match for an emoji, so "thumbs" does not find an umbrella.
func recordScore(fields []string, q searchQuery) int {
	best := 0
	for _, field := range fields {
		best = max(best, orderedScore(field, q))
		if best >= scoreExact {
			return best
		}
	}
	return best
}
