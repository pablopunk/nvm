package extview

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// searchChangeDebounce is how long the search text must rest before the view's onSearchChange handler runs. It is
// the palette's pause for typing, as Electron has it.
const searchChangeDebounce = 350 * time.Millisecond

// searchChange is the second batch of a list or grid view: the item ids its onSearchChange handler named for the text
// the user last paused on. The ids answer only that text, so a new text shows the local matches until its own answer
// arrives. asked is the text the handler was last scheduled for, and gen drops the timers and answers of texts already
// left. inFlight is true from the moment the handler is called for asked until its answer, so the palette shows the
// batch as loading; typing another text clears it.
type searchChange struct {
	asked    string
	query    string
	ids      []string
	items    []api.Item
	gen      int
	inFlight bool
}

// scheduleSearchChange asks the handler about query once it rests. Asking again for the text already asked does
// nothing, so a refresh of the same search never repeats the call.
func (s *Session) scheduleSearchChange(query string) {
	ref := s.view.OnSearchChange
	text := strings.TrimSpace(query)
	if ref == nil || s.deps.Clock == nil || text == s.searchChange.asked {
		return
	}
	s.searchChange.asked = text
	s.searchChange.gen++
	s.searchChange.inFlight = false
	if text == "" {
		s.searchChange.query, s.searchChange.ids, s.searchChange.items = "", nil, nil
		return
	}
	gen := s.searchChange.gen
	rest := s.deps.Clock.After(searchChangeDebounce)
	go func() {
		<-rest
		s.onUI(func() { s.runSearchChange(gen, *ref, text) })
	}()
}

// runSearchChange invokes the handler for text, unless the text has been left since the timer started. The batch is
// in flight until the handler answers or fails.
func (s *Session) runSearchChange(gen int, ref api.ActionRef, text string) {
	if gen != s.searchChange.gen {
		return
	}
	s.searchChange.inFlight = true
	s.refreshHost()
	s.invokeRaw(s.withAccessoryArg(ref), text, func(result api.Result, failure *api.Failure) {
		if gen != s.searchChange.gen {
			return
		}
		s.searchChange.inFlight = false
		s.searchChange.query, s.searchChange.ids, s.searchChange.items = text, nil, nil
		if failure == nil && result.Patch != nil {
			s.Apply(*result.Patch)
			s.searchChange.ids = result.Patch.SearchResultIds
			s.searchChange.items = result.Patch.SearchItems
		}
		s.refreshHost()
	})
}

// searchChangePending reports whether the handler's second batch for the shown text is still in flight.
func (s *Session) searchChangePending() bool {
	return s.searchChange.inFlight
}

// searchChangeRows returns the second batch for query: the items the handler named, in its order, skipping any row
// already in local and any id the view no longer holds. Their rows are not filtered by the query.
func (s *Session) searchChangeRows(local []search.Item, query string) []search.Item {
	if s.searchChange.query != strings.TrimSpace(query) {
		return nil
	}
	out := s.searchChangeIDRows(local)
	for _, item := range s.searchChange.items {
		if !itemSupported(s.deps, item) {
			continue
		}
		out = append(out, s.row(item))
	}
	return out
}

// searchChangeIDRows returns the rows of the batch's held ids that the local rows do not already show.
func (s *Session) searchChangeIDRows(local []search.Item) []search.Item {
	shown := make(map[string]bool, len(local))
	for _, row := range local {
		shown[row.ID] = true
	}
	held := s.heldItems()
	var out []search.Item
	for _, id := range s.searchChange.ids {
		item, ok := held[id]
		if !ok || shown[id] {
			continue
		}
		shown[id] = true
		if !itemSupported(s.deps, item) {
			continue
		}
		out = append(out, s.row(item))
	}
	return out
}

// heldItems indexes every item of the view by id, across its sections.
func (s *Session) heldItems() map[string]api.Item {
	held := make(map[string]api.Item, len(s.items))
	for _, item := range s.items {
		held[item.ID] = item
	}
	for _, section := range s.sections {
		for _, item := range section.Items {
			held[item.ID] = item
		}
	}
	return held
}
