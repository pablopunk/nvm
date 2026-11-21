package palette

import (
	"fmt"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestActiveViewFollowsPushPopAndVisibility proves the view on top hears when it becomes the active page of a
// shown palette, and the view it covers hears that it stopped being one. A camera relies on this to stop capturing
// while it is hidden or covered.
func TestActiveViewFollowsPushPopAndVisibility(t *testing.T) {
	var events []string
	record := func(name string) func(bool) {
		return func(active bool) { events = append(events, fmt.Sprintf("%s:%v", name, active)) }
	}
	root := &View{Title: "Root", Search: func(string) []search.Item { return nil }, OnActive: record("root")}
	h := newHarness(t, root, true)
	child := &View{Title: "Child", Search: func(string) []search.Item { return nil }, OnActive: record("child")}

	h.p.Push(child)
	h.p.Pop()
	h.p.Push(child)
	h.p.Hide()
	h.p.Show()

	want := []string{
		"root:true",
		"root:false", "child:true",
		"root:true",
		"root:false", "child:true",
		"child:false",
		"root:true",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("active events = %v, want %v", events, want)
	}
}
