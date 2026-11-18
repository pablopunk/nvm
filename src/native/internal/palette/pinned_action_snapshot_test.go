package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestPinnedActionsRemainPresentedUntilRedraw(t *testing.T) {
	for _, input := range []string{"enter", "shortcut", "menu"} {
		t.Run(input, func(t *testing.T) {
			var ran string
			action := func(name string) search.Action {
				return search.Action{Title: name, Key: ui.KeyY, Mods: Mod, Dismiss: search.DismissNever, Run: func(search.Host) { ran = name }}
			}
			view := &View{Title: "Root"}
			p := New(Options{Root: view})
			p.query = "q"
			rows := []search.Item{{ID: "other"}, {ID: "chosen", Actions: []search.Action{action("shown")}}}
			p.showRows(view, "q", rows, true, false)
			p.Choose(1)
			p.recordPresentedChoice()
			rows[1].Actions[0] = action("changed")
			run := func() {
				switch input {
				case "enter":
					p.onEnter()
				case "shortcut":
					tester := ui.NewTester(p.runActionShortcut, DefaultWidth, DefaultHeight)
					tester.Key(Mod, ui.KeyY)
				case "menu":
					p.openActionMenu(0)
					p.runMenuAction(0)
				}
			}
			run()
			if ran != "shown" {
				t.Fatalf("before redraw ran %q", ran)
			}
			p.recordPresentedChoice()
			run()
			if ran != "changed" {
				t.Fatalf("after redraw ran %q", ran)
			}
		})
	}
}

func TestExplicitMoveUsesNewChoiceRatherThanPriorPresentedAction(t *testing.T) {
	var ran string
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.query = "q"
	var rows []search.Item
	for _, title := range []string{"first", "second"} {
		rows = append(rows, search.Item{ID: title, Actions: []search.Action{{Title: title, Dismiss: search.DismissNever, Run: func(search.Host) { ran = title }}}})
	}
	p.showRows(view, "q", rows, true, false)
	p.Choose(0)
	p.recordPresentedChoice()
	p.Choose(1)
	p.onEnter()
	if ran != "second" {
		t.Fatalf("explicit move ran %q", ran)
	}
}
