package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestListReloadsItsRowsWhenThePageAboveItPops: a list with a refresh handler runs it when it is revealed, so a row
// saved on a form above it shows without a new search.
func TestListReloadsItsRowsWhenThePageAboveItPops(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{
		"reload": {Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "b", Title: "Bravo"}}}},
	}}
	view := listItems(api.Item{ID: "a", Title: "Alpha"})
	view.Refresh = &api.Refresh{Ref: &api.ActionRef{ID: "reload"}, Mode: api.PatchReplace}
	s := NewSession(log.deps(nil), "acme", view)
	page := s.listPage()
	if page.OnReveal == nil {
		t.Fatal("a list with a refresh handler does not reload when it is revealed")
	}
	page.OnReveal()
	if len(log.calls) != 1 || log.calls[0] != "acme:reload" {
		t.Fatalf("calls = %v, want the refresh handler once", log.calls)
	}
	if got := rowTitles(s.rows("")); len(got) != 1 || got[0] != "Bravo" {
		t.Fatalf("rows after the reveal = %v, want Bravo", got)
	}
}
