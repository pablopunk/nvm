package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Replace swaps the content of a list view for view, as a refresh of a preview does: its items, sections, empty state
// and failure change, and the page the session draws stays the same. The next search reads the new items.
func (s *Session) Replace(view api.View) {
	s.view = view
	s.items, s.sections = view.Items, view.Sections
	s.hasMore = view.Pagination != nil && view.Pagination.HasMore && view.Pagination.Next != nil
	s.failure, s.loading = nil, false
}
