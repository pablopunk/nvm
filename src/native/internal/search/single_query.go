package search

import (
	"slices"
	"strings"
)

// singleQueryState keeps the displayed snapshot apart from refresh work. Only a matching, uniquely identified
// leading row carries across a narrowing; this is independent of subsequent ranking changes.
type singleQueryState struct {
	visible  *Snapshot
	previous *Snapshot
	seed     NarrowingSeedProvider
}

func (s *singleQueryState) begin(q string) {
	if s.visible != nil && s.visible.Query != q {
		s.previous = s.visible
	}
	s.visible = nil
	s.seed = nil
}

func (s *singleQueryState) setVisible(snapshot Snapshot) {
	copy := snapshot
	copy.Items = slices.Clone(snapshot.Items)
	s.visible = &copy
}

func (s *singleQueryState) refine(q string, rows []Item) []Item {
	if s.previous == nil || len(s.previous.Items) == 0 || s.previous.Query == "" || q == s.previous.Query || !strings.HasPrefix(q, s.previous.Query) {
		return rows
	}
	old := s.previous.Items[0]
	if old.ID == "" || old.QueryFallback {
		return rows
	}
	match := MatchesName(q, old.Title)
	for _, name := range old.Names {
		match = match || MatchesName(q, name)
	}
	if !match {
		return rows
	}
	found := -1
	for i, row := range rows {
		if row.ID == old.ID && row.Kind == old.Kind && row.Provider == old.Provider && row.Source == old.Source {
			if found >= 0 {
				return rows
			}
			found = i
		}
	}
	if found < 0 && s.seed != nil {
		admitted := s.seed.SeedNarrowing(q, []Item{old})
		if len(admitted) == 1 && admitted[0].ID == old.ID && admitted[0].Kind == old.Kind && admitted[0].Provider == old.Provider && admitted[0].Source == old.Source {
			rows = append(slices.Clone(rows), admitted[0])
			found = len(rows) - 1
		}
	}
	if found <= 0 {
		return rows
	}
	out := slices.Clone(rows)
	item := out[found]
	copy(out[1:found+1], out[:found])
	out[0] = item
	return out
}
