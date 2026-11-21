package extview

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

type navigationHost struct {
	stack  []*palette.View
	query  string
	toasts []string
}

func (h *navigationHost) Hide() {}
func (h *navigationHost) Push(view any) {
	h.stack = append(h.stack, view.(*palette.View))
	h.query = ""
}
func (h *navigationHost) Pop() {
	if len(h.stack) > 1 {
		v := h.stack[len(h.stack)-1]
		if v.OnLeave != nil {
			v.OnLeave()
		}
		h.stack = h.stack[:len(h.stack)-1]
		h.query = ""
	}
}
func (h *navigationHost) Root() {
	for len(h.stack) > 1 {
		h.Pop()
	}
}
func (h *navigationHost) Toast(message string)  { h.toasts = append(h.toasts, message) }
func (h *navigationHost) Refresh()              {}
func (h *navigationHost) Query() string         { return h.query }
func (h *navigationHost) SetQuery(query string) { h.query = query }

func TestDeclarativeViewNavigationPreservesStackAndCancelsOldWork(t *testing.T) {
	for _, tc := range []struct {
		kind  api.ActionKind
		depth int
	}{{api.ActionReplaceView, 3}, {api.ActionRootView, 2}} {
		t.Run(string(tc.kind), func(t *testing.T) {
			h := &navigationHost{stack: []*palette.View{{Title: "Palette"}}}
			parent := NewSession(Deps{}, "acme", api.ListView("Parent"))
			h.Push(parent.Open(h))
			old := parent.navigationChild("acme", api.ListView("Old"), api.NavPush)
			h.Push(old.Open(h))
			h.query = "old filter"
			destination := api.ListView("Destination", api.Item{ID: "target", Title: "Target"})
			action := old.action(api.Action{Kind: tc.kind, Title: "Navigate", View: &destination}, "x")
			if action.Dismiss != search.DismissNever {
				t.Fatal("navigation dismisses palette")
			}
			action.Run(h)
			if len(h.stack) != tc.depth || h.stack[len(h.stack)-1].Title != "Destination" || h.query != "" {
				t.Fatalf("stack %v query %q", h.stack, h.query)
			}
			if old.workContext.Err() != context.Canceled {
				t.Fatal("old view work was not canceled")
			}
			if tc.kind == api.ActionRootView && parent.workContext.Err() != context.Canceled {
				t.Fatal("root did not close parent")
			}
			h.Pop()
			want := "Parent"
			if tc.kind == api.ActionRootView {
				want = "Palette"
			}
			if h.stack[len(h.stack)-1].Title != want {
				t.Fatalf("back landed on %q, want %q", h.stack[len(h.stack)-1].Title, want)
			}
		})
	}
}

func TestDeclarativeViewNavigationRejectsInvalidAndHonorsConfirmation(t *testing.T) {
	h := &navigationHost{stack: []*palette.View{{Title: "Palette"}}}
	s := NewSession(Deps{}, "acme", api.ListView("Parent"))
	h.Push(s.Open(h))
	bad := api.View{Kind: api.ViewGrid, Title: "Broken"}
	s.action(api.Action{Kind: api.ActionReplaceView, View: &bad}, "bad").Run(h)
	if len(h.stack) != 2 || len(h.toasts) != 1 {
		t.Fatalf("invalid destination navigated: %+v", h)
	}
	dest := api.ListView("Confirmed")
	a := api.Action{Kind: api.ActionRootView, Title: "Reset", View: &dest, Confirm: &api.Confirmation{Title: "Continue?"}}
	s.action(a, "confirm").Run(h)
	if len(h.stack) != 3 || h.stack[2].Title != "Confirm" {
		t.Fatal("confirmation was skipped")
	}
	h.stack[2].Search("")[0].Actions[0].Run(h)
	if len(h.stack) != 2 || h.stack[1].Title != "Confirmed" {
		t.Fatalf("confirmed navigation: %+v", h.stack)
	}
}

func TestDeclarativeViewNavigationFromFollowUp(t *testing.T) {
	h := &navigationHost{stack: []*palette.View{{Title: "Palette"}}}
	s := NewSession(Deps{}, "acme", api.ListView("Parent"))
	h.Push(s.Open(h))
	dest := api.ListView("Follow up")
	s.finish(h, "acme", "open", api.Result{Next: &api.Action{Kind: api.ActionReplaceView, View: &dest}}, nil)
	if len(h.stack) != 2 || h.stack[1].Title != "Follow up" {
		t.Fatalf("follow-up navigation: %+v", h.stack)
	}
}
