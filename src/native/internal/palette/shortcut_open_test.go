package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

type openingWindow struct {
	*fakeWindow
	palette *Palette
	shown   []*View
}

func (w *openingWindow) Show() {
	w.shown = append(w.shown, w.palette.Top())
	w.fakeWindow.Show()
}

func TestShortcutOpensItsTargetBeforeTheWindowIsShown(t *testing.T) {
	rootSearches := 0
	root := &View{Title: "root", Search: func(string) []search.Item { rootSearches++; return nil }}
	p := New(Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	w := &openingWindow{fakeWindow: newFakeWindow(), palette: p}
	p.Attach(w)
	target := &View{Title: "Clipboard History"}
	p.Open(target)
	if len(w.shown) != 1 || w.shown[0] != target || rootSearches != 0 {
		t.Fatalf("opening showed the root: shown=%v root searches=%d", w.shown, rootSearches)
	}
}

func TestShortcutLoadingIsTargetSpecificAndReplacedInHistory(t *testing.T) {
	root := &View{Title: "root"}
	p := New(Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	w := &openingWindow{fakeWindow: newFakeWindow(), palette: p}
	p.Attach(w)
	var finish func() bool
	p.OpenAction("Clipboard History", search.Action{Dismiss: search.DismissNever, Run: func(h search.Host) { finish = h.(search.ActionLoadingHost).BeginAction() }})
	opening := p.Top()
	if len(w.shown) != 1 || w.shown[0] != opening || opening.Title != "Clipboard History" || !p.loadingActionOrView() {
		t.Fatal("shortcut did not show its named loading surface directly")
	}
	if !finish() {
		t.Fatal("showing the shortcut invalidated its action completion")
	}
	target := &View{Title: "Clipboard History", Search: func(string) []search.Item { return []search.Item{{Title: "a copy"}} }}
	p.Push(target)
	if len(p.stack) != 2 || p.Top() != target || p.stack[0] != root || p.loadingActionOrView() {
		t.Fatal("loading surface remained in navigation history")
	}
	p.Pop()
	if p.Visible() {
		t.Fatal("Back from an independent shortcut entry did not dismiss the palette")
	}
	p.Show()
	if p.Top() != root {
		t.Fatal("summoning after shortcut dismissal did not restore the root")
	}
}

func TestShortcutCompletionCannotReplaceDismissedOrNewerDestinations(t *testing.T) {
	for _, leave := range []string{"dismiss", "summon", "shortcut", "back"} {
		t.Run(leave, func(t *testing.T) {
			p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true})
			p.Attach(newFakeWindow())
			var finish func() bool
			p.OpenAction("Clipboard History", search.Action{Dismiss: search.DismissNever, Run: func(h search.Host) { finish = h.(search.ActionLoadingHost).BeginAction() }})
			switch leave {
			case "dismiss":
				p.Hide()
			case "summon":
				p.Show()
			case "shortcut":
				p.OpenAction("Notes", search.Action{Run: func(h search.Host) { h.Push(&View{Title: "Notes"}) }})
			case "back":
				p.Pop()
			}
			if finish() {
				t.Fatal("obsolete shortcut completion was accepted")
			}
		})
	}
}

func TestShortcutKeepsTheChatThatSurvivedDismissal(t *testing.T) {
	root := &View{Title: "root"}
	p := New(Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(newFakeWindow())
	left := false
	chat := &View{Title: "chat", SurvivesHide: true, OnLeave: func() { left = true }}
	p.Push(chat)
	p.Hide()
	target := &View{Title: "Clipboard History"}
	p.OpenAction(target.Title, search.Action{Run: func(h search.Host) { h.Push(target) }})
	if left || len(p.stack) != 3 || p.Top() != target {
		t.Fatal("shortcut discarded the surviving chat")
	}
	p.Pop()
	if p.Visible() {
		t.Fatal("Back from an independent shortcut entry did not dismiss the palette")
	}
	p.Show()
	if p.Top() != chat || left {
		t.Fatal("summoning after shortcut dismissal did not restore the surviving chat")
	}
}
