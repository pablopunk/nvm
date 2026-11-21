package files

import "time"

// SortBy names the key results are ordered by. The zero value leaves the snapshot order, which is not stable
// across rebuilds; set a key whenever the order matters.
type SortBy string

// Sort keys, named as the extension API names them.
const (
	SortNone     SortBy = ""
	SortAdded    SortBy = "added"
	SortCreated  SortBy = "created"
	SortRecent   SortBy = "recent"
	SortModified SortBy = "modified"
	SortName     SortBy = "name"
	SortSize     SortBy = "size"
)

// Order is the direction of a sort. The zero value is descending, which is what the palette wants for recency.
type Order uint8

// Sort directions.
const (
	OrderDesc Order = iota
	OrderAsc
)

// DefaultLimit is the number of results a query returns when it sets no limit.
const DefaultLimit = 100

// MaxLimit caps the results of one query, as file-index.ts caps its limit.
const MaxLimit = 20_000

// Query selects files from a snapshot. The zero value selects every file, in snapshot order, up to DefaultLimit.
// Field names follow the extension API's find options.
type Query struct {
	// Text is matched against file names. An empty text selects every file.
	Text string
	// Roots restricts results to these folders and everything under them. Empty means every indexed root.
	Roots []string
	// Kinds keeps files whose extension belongs to one of these kinds. KindFile adds no restriction.
	Kinds []Kind
	// Extensions keeps files with these extensions, with or without a leading dot.
	Extensions []string
	// Patterns keeps files whose pattern ends in an extension, such as "*.pdf".
	Patterns []string
	// Ignore excludes entries matching these patterns in addition to the index's own ignore rules.
	Ignore []string
	// Limit is the number of results to return. Zero means DefaultLimit; values above MaxLimit are capped.
	Limit  int
	SortBy SortBy
	Order  Order
}

// Result is one matching file.
type Result struct {
	Path        string
	DisplayPath string
	Name        string
	Extension   string
	Kind        Kind
	Size        int64
	Modified    time.Time
	Created     time.Time
	// Score is the name-match band of the result, 0 when the query had no text.
	Score int
}

// Batch is one streamed step of a search. Each batch holds the best results found so far, best first; a later batch
// replaces an earlier one. The last batch of a search that was not cancelled has Final set.
type Batch struct {
	Results []Result
	Final   bool
}

// effectiveLimit applies the default and the cap to a query limit.
func effectiveLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultLimit
	case limit > MaxLimit:
		return MaxLimit
	default:
		return limit
	}
}
