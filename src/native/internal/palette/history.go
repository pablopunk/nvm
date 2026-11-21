package palette

import (
	"slices"
	"strings"
)

// QueryHistoryLimit is how many distinct submitted queries the root remembers (1.x QUERY_HISTORY_LIMIT).
const QueryHistoryLimit = 100

// QueryHistory is the root's memory of the queries the user submitted, oldest first, and the cursor that
// ArrowUp and ArrowDown walk through it. It follows 1.x App.tsx: a query is remembered when a row at the root
// runs or a query goes to the AI chat; a query that is submitted again moves to the end; typing ends a walk
// and makes the typed text the draft that the walk returns to. Persist, when set, receives the entries after
// each change, so the host can save them.
type QueryHistory struct {
	entries []string
	persist func([]string)
	cursor  int // the recalled entry, or -1 when the field shows what the user typed
	draft   string
}

// NewQueryHistory returns the history of the saved queries. Blank and repeated entries are dropped, keeping
// the last occurrence, and only the newest QueryHistoryLimit are kept.
func NewQueryHistory(saved []string, persist func([]string)) *QueryHistory {
	return &QueryHistory{entries: normalizeHistory(saved), persist: persist, cursor: -1}
}

// normalizeHistory returns the usable entries of saved, oldest first.
func normalizeHistory(saved []string) []string {
	seen := map[string]bool{}
	var out []string
	for i := len(saved) - 1; i >= 0; i-- {
		q := strings.TrimSpace(saved[i])
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	slices.Reverse(out)
	return trimToLimit(out)
}

// trimToLimit keeps the newest QueryHistoryLimit entries.
func trimToLimit(entries []string) []string {
	if over := len(entries) - QueryHistoryLimit; over > 0 {
		return entries[over:]
	}
	return entries
}

// Entries returns a copy of the remembered queries, oldest first.
func (h *QueryHistory) Entries() []string { return slices.Clone(h.entries) }

// Remember records a submitted query. A blank query is not remembered and does not end a walk, as 1.x does.
func (h *QueryHistory) Remember(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	h.cursor, h.draft = -1, ""
	h.entries = append(slices.DeleteFunc(h.entries, func(e string) bool { return e == q }), q)
	h.entries = trimToLimit(h.entries)
	if h.persist != nil {
		h.persist(slices.Clone(h.entries))
	}
}

// Reset ends a walk: the next ArrowUp takes the text on the field as the new draft. Typing calls it.
func (h *QueryHistory) Reset() {
	h.cursor, h.draft = -1, ""
}

// Navigating reports whether the field shows a recalled query.
func (h *QueryHistory) Navigating() bool { return h.cursor >= 0 }

// Older steps back one query (ArrowUp) and returns it. The first step remembers current as the draft. It
// reports false when there is nothing to recall.
func (h *QueryHistory) Older(current string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	switch {
	case h.cursor < 0:
		h.draft = current
		h.cursor = len(h.entries) - 1
	case h.cursor > 0:
		h.cursor--
	}
	return h.entries[h.cursor], true
}

// Newer steps forward one query (ArrowDown) and returns it. Past the newest query it returns the draft and
// ends the walk. It reports false when no walk is under way.
func (h *QueryHistory) Newer() (string, bool) {
	if h.cursor < 0 {
		return "", false
	}
	h.cursor++
	if h.cursor >= len(h.entries) {
		h.cursor = -1
		return h.draft, true
	}
	return h.entries[h.cursor], true
}

// Restore ends a walk (Escape) and returns the text the user typed before it. It reports false when no walk
// is under way.
func (h *QueryHistory) Restore() (string, bool) {
	if h.cursor < 0 {
		return "", false
	}
	h.cursor = -1
	return h.draft, true
}
