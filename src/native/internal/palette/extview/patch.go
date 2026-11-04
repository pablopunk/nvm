package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Apply changes the visible view as a result's patch says. Removals come first. A patch replaces the items with the
// same ids, an unknown id is dropped rather than appended, and replace, prepend and append change the whole list.
// Sections take the same patch, each on its own items.
func (s *Session) Apply(patch api.ViewPatch) {
	if patch.IsLoading != nil {
		s.loading = *patch.IsLoading
	}
	if len(s.sections) > 0 {
		for i := range s.sections {
			s.sections[i].Items = applyPatch(s.sections[i].Items, patch)
		}
		return
	}
	s.items = applyPatch(s.items, patch)
}

// applyPatch returns items changed by patch. It never mutates its input.
func applyPatch(items []api.Item, patch api.ViewPatch) []api.Item {
	kept := withoutIDs(items, patch.RemoveItemIDs)
	switch patch.Mode {
	case api.PatchReplace:
		return append([]api.Item(nil), patch.Items...)
	case api.PatchPrepend:
		return append(append([]api.Item(nil), patch.Items...), withoutItemsIn(kept, patch.Items)...)
	case api.PatchAppend:
		return append(append([]api.Item(nil), withoutItemsIn(kept, patch.Items)...), patch.Items...)
	}
	return mergeInPlace(kept, patch.Items)
}

// mergeInPlace replaces each item whose id is in updates, keeps the others in order, and drops updates that match
// nothing.
func mergeInPlace(items, updates []api.Item) []api.Item {
	byID := make(map[string]api.Item, len(updates))
	for _, update := range updates {
		byID[update.ID] = update
	}
	out := make([]api.Item, 0, len(items))
	for _, item := range items {
		if update, ok := byID[item.ID]; ok {
			out = append(out, update)
		} else {
			out = append(out, item)
		}
	}
	return out
}

func withoutIDs(items []api.Item, ids []string) []api.Item {
	if len(ids) == 0 {
		return items
	}
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	out := make([]api.Item, 0, len(items))
	for _, item := range items {
		if !drop[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

// withoutItemsIn drops the items whose ids appear in updates, so an updated item is not listed twice.
func withoutItemsIn(items, updates []api.Item) []api.Item {
	ids := make([]string, 0, len(updates))
	for _, update := range updates {
		ids = append(ids, update.ID)
	}
	return withoutIDs(items, ids)
}
