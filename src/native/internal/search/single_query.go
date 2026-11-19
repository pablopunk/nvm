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

// rowIdentity names a row across refreshes of one query: its source and its ID, or its title without an ID.
type rowIdentity struct{ provider, source, kind, id, title, learnKey string }

func identityOf(item Item) rowIdentity {
	key := rowIdentity{provider: item.Provider, source: item.Source, kind: item.Kind, id: item.ID}
	if item.ID == "" {
		key.title, key.learnKey = item.Title, item.LearnKey
	}
	return key
}

// updateInPlace keeps the rows on screen and their order, and takes each row's content from the refresh when the
// refresh still has exactly one row of that identity: a status, an icon or an action list that arrived later. A row
// the refresh no longer has stays as it was, and a row it adds waits for the next query, so the set never changes
// under the user. An updated row keeps its publication, so an action bound to the row on screen still finds it.
func updateInPlace(shown, fresh []Item) []Item {
	byIdentity := make(map[rowIdentity]int, len(fresh))
	for i, item := range fresh {
		key := identityOf(item)
		if _, seen := byIdentity[key]; seen {
			byIdentity[key] = -1
			continue
		}
		byIdentity[key] = i
	}
	out := slices.Clone(shown)
	for i, row := range out {
		if j, ok := byIdentity[identityOf(row)]; ok && j >= 0 {
			updated := fresh[j]
			updated.Publication = row.Publication
			out[i] = updated
		}
	}
	return out
}

// sameDisplay reports whether two results draw the same rows: the same rows in the same order, with the same text,
// decorations and actions. Closures and decoded images are compared by presence only.
func sameDisplay(a, b []Item) bool {
	return slices.EqualFunc(a, b, func(x, y Item) bool {
		return identityOf(x) == identityOf(y) && displayOf(x) == displayOf(y) && slices.EqualFunc(x.Actions, y.Actions, sameAction)
	})
}

type rowDisplay struct {
	title, subtitle, glyph, icon, section, tone, iconPath, iconFallback, imageShape, accessory string
	prominent, tintTitle, disabled, contextual, image, detail                                  bool
}

func displayOf(item Item) rowDisplay {
	return rowDisplay{
		title: item.Title, subtitle: item.Subtitle, glyph: item.Glyph, icon: item.Icon, section: item.Section, tone: item.Tone,
		iconPath: item.IconPath, iconFallback: item.IconFallback, imageShape: item.ImageShape, accessory: item.Accessory,
		prominent: item.Prominent, tintTitle: item.TintTitle, disabled: item.Disabled, contextual: item.Contextual,
		image: item.Image != nil, detail: item.Detail != nil,
	}
}

func sameAction(a, b Action) bool {
	return a.Title == b.Title && a.Subtitle == b.Subtitle && a.Mods == b.Mods && a.Key == b.Key && a.Label == b.Label &&
		a.Dismiss == b.Dismiss && a.Disabled == b.Disabled && a.MenuHidden == b.MenuHidden && a.Section == b.Section &&
		a.MenuOnly == b.MenuOnly && (a.Run == nil) == (b.Run == nil)
}

// SameDisplay reports whether two results draw the same rows, as a single-snapshot refresh compares them.
func SameDisplay(a, b []Item) bool { return sameDisplay(a, b) }
