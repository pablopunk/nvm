package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestDisabledMenuActionNeitherRunsNorClosesTheMenu chooses a disabled action with Enter: nothing runs and the menu stays
// open, so the reason it shows stays on screen.
func TestDisabledMenuActionNeitherRunsNorClosesTheMenu(t *testing.T) {
	var ran []string
	view := &View{Title: "Root", Placeholder: "Search", Search: func(string) []search.Item {
		return []search.Item{{Kind: "Test", Title: "Alpha", Actions: []search.Action{
			{Title: "Primary", Run: func(search.Host) { ran = append(ran, "primary") }},
			{Title: "Rename", Disabled: "Rename is not in 2.0 yet", Run: func(search.Host) { ran = append(ran, "rename") }},
		}}}
	}}
	h := newHarness(t, view, true)
	h.typeText("al")
	h.key(Mod, ui.KeyK)
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyEnter)
	if len(ran) != 0 {
		t.Fatalf("a disabled action ran: %v", ran)
	}
	if !h.p.actionsOpen {
		t.Fatal("choosing a disabled action closed the menu")
	}
}
