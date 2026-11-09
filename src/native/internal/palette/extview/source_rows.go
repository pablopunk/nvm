package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// loadMoreRow is the pagination row at the end of the list. Its action asks the extension for the next page.
func (s *Session) loadMoreRow() search.Item {
	next := *s.view.Pagination.Next
	return search.Item{
		ID: loadMoreID, Title: "Load more", Icon: "plus", Tone: extensionTone,
		Actions: []search.Action{{Title: "Load more", Dismiss: search.DismissNever, Run: func(h search.Host) { s.loadMore(h, next) }}},
	}
}

// loadMore appends the next page. An empty page, or one without a patch, is the end of the list.
func (s *Session) loadMore(h search.Host, next api.ActionRef) {
	revision := s.itemsRevision
	s.invokeRaw(next, "", func(result api.Result, failure *api.Failure) {
		if s.dismissed || revision != s.itemsRevision {
			return
		}
		if failure != nil {
			h.Toast(failure.Message)
			return
		}
		if result.Patch == nil || len(result.Patch.Items) == 0 {
			s.hasMore = false
			h.Refresh()
			return
		}
		patch := *result.Patch
		if patch.Mode == "" {
			patch.Mode = api.PatchAppend
		}
		s.Apply(patch)
		h.Refresh()
	})
}

// failureRows is what a failed load shows above the items it kept: the reason, and a retry row when the source allows it.
func (s *Session) failureRows() []search.Item {
	rows := []search.Item{{
		ID: failedID, Title: "Could not load these items", Subtitle: s.failure.Message,
		Contextual: true, Disabled: true,
	}}
	if s.view.Source != nil && s.view.Source.Retry {
		rows = append(rows, search.Item{
			ID: retryID, Title: "Try again", Icon: "rotate-ccw", Tone: extensionTone,
			Actions: []search.Action{{Title: "Try again", Dismiss: search.DismissNever, Run: func(search.Host) { s.reload() }}},
		})
	}
	return rows
}
