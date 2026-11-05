package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPendingActionAcknowledgesImmediatelyAndRejectsObsoleteCompletion(t *testing.T) {
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true})
	first := p.BeginAction()
	if !p.loadingActionOrView() {
		t.Fatal("the action had no immediate loading feedback")
	}
	second := p.BeginAction()
	if first() || !p.loadingActionOrView() {
		t.Fatal("an older action cleared the newer action's feedback")
	}
	if !second() || p.loadingActionOrView() || second() {
		t.Fatal("completion was not accepted exactly once and cleared")
	}
	changedQuery := p.BeginAction()
	p.SetQuery("another command")
	if changedQuery() || p.loadingActionOrView() {
		t.Fatal("changing the query did not invalidate the pending action")
	}
	navigated := p.BeginAction()
	p.SetStack(&View{Title: "another view"})
	if navigated() || p.loadingActionOrView() {
		t.Fatal("leaving the view did not invalidate the pending action")
	}
}
