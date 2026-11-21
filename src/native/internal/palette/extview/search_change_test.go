package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// searchRig is a list view whose onSearchChange handler answers each text from answers. Every invocation is recorded
// with the text it was asked about, and the UI queue stands in for the palette's thread.
type searchRig struct {
	t       *testing.T
	session *Session
	clock   *fake.Clock
	ui      *uiQueue
	host    *fakeHost
	asked   []string
	answers map[string][]string
	failure *api.Failure
	// hold keeps each answer until releaseHeld, so a test sees the batch while the handler runs.
	hold bool
	held []func()
}

func newSearchRig(t *testing.T, view api.View, answers map[string][]string) *searchRig {
	t.Helper()
	rig := &searchRig{t: t, clock: fake.NewClock(), ui: newUIQueue(), host: &fakeHost{}, answers: answers}
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			rig.asked = append(rig.asked, in.Value)
			answer := func() { rig.answerWith(in.Value, done) }
			if rig.hold {
				rig.held = append(rig.held, answer)
				return
			}
			answer()
		},
		OnUI:  rig.ui.post,
		Clock: rig.clock,
	}
	rig.session = NewSession(deps, "acme.views", view)
	rig.session.Open(rig.host)
	return rig
}

// answerWith calls done as the handler answers text: with its failure when one is set, else with the ids for text.
func (r *searchRig) answerWith(text string, done func(api.Result, *api.Failure)) {
	if r.failure != nil {
		done(api.Result{}, r.failure)
		return
	}
	done(api.Result{Patch: &api.ViewPatch{SearchResultIds: r.answers[text]}}, nil)
}

// releaseHeld answers every held call, oldest first.
func (r *searchRig) releaseHeld() {
	held := r.held
	r.held = nil
	for _, answer := range held {
		answer()
	}
}

// pendingNow reports whether the view's second batch is shown as loading.
func (r *searchRig) pendingNow() bool {
	return r.session.Palette().Pending()
}

// rest moves the clock past the debounce and runs the callbacks that the timers posted, as the UI thread would.
func (r *searchRig) rest(posts int) {
	r.clock.Advance(searchChangeDebounce)
	for range posts {
		r.ui.settle(r.t)
	}
}

func searchView(onSearchChange bool, items ...api.Item) api.View {
	view := api.View{Kind: api.ViewList, Title: "Cats", Items: items}
	if onSearchChange {
		view.OnSearchChange = &api.ActionRef{ID: "meaning"}
	}
	return view
}

var catItems = []api.Item{
	{ID: "cat", Title: "Cat"},
	{ID: "tiger", Title: "Tiger"},
	{ID: "whale", Title: "Whale"},
}

func TestSecondBatchFollowsTheLocalMatchesOnceTheTextRests(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger", "cat"}})
	if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 1 || got[0] != "Cat" {
		t.Fatalf("rows before the pause = %v, want the local match only", got)
	}
	rig.rest(1)
	if len(rig.asked) != 1 || rig.asked[0] != "ca" {
		t.Fatalf("asked = %v, want the handler asked once about ca", rig.asked)
	}
	if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 2 || got[0] != "Cat" || got[1] != "Tiger" {
		t.Fatalf("rows after the answer = %v, want Cat then Tiger without repeating Cat", got)
	}
	if rig.host.refresh == 0 {
		t.Fatal("the answer did not refresh the palette")
	}
}

func TestSecondBatchIsNotFilteredByTheQuery(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"cute": {"whale"}})
	rig.session.Palette().Search("cute")
	rig.rest(1)
	if got := rowTitles(rig.session.Palette().Search("cute")); len(got) != 1 || got[0] != "Whale" {
		t.Fatalf("rows = %v, want the second-batch Whale although the title does not match", got)
	}
}

func TestAnAnswerForTextThatChangedIsDropped(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}, "dog": nil})
	rig.session.Palette().Search("ca")
	rig.session.Palette().Search("dog")
	rig.rest(2)
	if len(rig.asked) != 1 || rig.asked[0] != "dog" {
		t.Fatalf("asked = %v, want only the text the user rests on", rig.asked)
	}
	if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 1 || got[0] != "Cat" {
		t.Fatalf("rows for ca = %v, want the local match only", got)
	}
}

func TestTextAlreadyAskedIsNotAskedAgain(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}})
	rig.session.Palette().Search("ca")
	rig.rest(1)
	rig.session.Palette().Search("ca")
	rig.clock.Advance(searchChangeDebounce)
	if len(rig.asked) != 1 {
		t.Fatalf("asked = %v, want one call for a repeated search", rig.asked)
	}
}

func TestEmptyTextNeverAsksAndClearsTheBatch(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}})
	rig.session.Palette().Search("ca")
	rig.rest(1)
	rig.session.Palette().Search("   ")
	rig.rest(0)
	if len(rig.asked) != 1 {
		t.Fatalf("asked = %v, want the empty text never asked", rig.asked)
	}
	if got := rowTitles(rig.session.Palette().Search("")); len(got) != 3 {
		t.Fatalf("rows = %v, want every item and no second batch", got)
	}
}

func TestFailedSearchChangeAddsNothingAndSaysNothing(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}})
	rig.failure = &api.Failure{Code: api.CodeInternal, Message: "the model is busy"}
	rig.session.Palette().Search("ca")
	rig.rest(1)
	if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 1 || got[0] != "Cat" {
		t.Fatalf("rows = %v, want the local match only after a failure", got)
	}
	if len(rig.host.toasts) != 0 {
		t.Fatalf("toasts = %v, a failed enrichment must stay silent", rig.host.toasts)
	}
}

func TestViewWithoutTheHandlerNeverAsks(t *testing.T) {
	rig := newSearchRig(t, searchView(false, catItems...), map[string][]string{"ca": {"tiger"}})
	rig.session.Palette().Search("ca")
	rig.rest(0)
	if len(rig.asked) != 0 {
		t.Fatalf("asked = %v, want no call without onSearchChange", rig.asked)
	}
}

func TestSecondBatchTakesItsRoomFromMaxVisibleItems(t *testing.T) {
	view := searchView(true, api.Item{ID: "cat", Title: "Cat"}, api.Item{ID: "catfish", Title: "Catfish"},
		api.Item{ID: "cathedral", Title: "Cathedral"}, api.Item{ID: "whale", Title: "Whale"})
	view.MaxVisibleItems = 2
	rig := newSearchRig(t, view, map[string][]string{"cat": {"whale"}})
	rig.session.Palette().Search("cat")
	rig.rest(1)
	got := rowTitles(rig.session.Palette().Search("cat"))
	if len(got) != 2 || got[0] != "Cat" || got[1] != "Whale" {
		t.Fatalf("rows = %v, want the first local match and the second batch within the cap", got)
	}
}

func TestSearchChangeNeedsAClockToDebounce(t *testing.T) {
	asked := 0
	deps := Deps{Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) { asked++ }}
	session := NewSession(deps, "acme.views", searchView(true, catItems...))
	session.Open(&fakeHost{})
	session.Palette().Search("ca")
	if asked != 0 {
		t.Fatalf("asked = %d, want no call without a clock to debounce on", asked)
	}
}

// TestTheSecondBatchIsLoadingOnlyWhileTheHandlerRuns checks the batch is pending from the moment its call starts, after
// the pause, until the answer lands.
func TestTheSecondBatchIsLoadingOnlyWhileTheHandlerRuns(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}})
	rig.hold = true
	rig.session.Palette().Search("ca")
	if rig.pendingNow() {
		t.Fatal("the batch is loading before the text rested")
	}
	rig.rest(1)
	if !rig.pendingNow() {
		t.Fatal("the batch is not loading while the handler runs")
	}
	rig.releaseHeld()
	if rig.pendingNow() {
		t.Fatal("the batch still loads after the answer")
	}
	if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 2 || got[1] != "Tiger" {
		t.Fatalf("rows = %v, want the answer shown in place of the placeholder", got)
	}
}

// TestAFailedOrEmptyBatchEndsLoadingWithoutRows checks a failed call and an empty answer both end the loading state
// and leave the local matches alone.
func TestAFailedOrEmptyBatchEndsLoadingWithoutRows(t *testing.T) {
	for name, configure := range map[string]func(*searchRig){
		"failure": func(r *searchRig) { r.failure = &api.Failure{Code: api.CodeInternal, Message: "busy"} },
		"empty":   func(r *searchRig) { r.answers["ca"] = nil },
	} {
		t.Run(name, func(t *testing.T) {
			rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}})
			configure(rig)
			rig.hold = true
			rig.session.Palette().Search("ca")
			rig.rest(1)
			if !rig.pendingNow() {
				t.Fatal("the batch is not loading while the handler runs")
			}
			rig.releaseHeld()
			if rig.pendingNow() {
				t.Fatal("the loading state stayed after the call ended")
			}
			if got := rowTitles(rig.session.Palette().Search("ca")); len(got) != 1 || got[0] != "Cat" {
				t.Fatalf("rows = %v, want the local match only", got)
			}
		})
	}
}

// TestTypingANewTextCancelsTheLoadingBatchAndRestartsIt checks a new text ends the loading state at once, drops the
// answer for the text left, and asks again after its own pause.
func TestTypingANewTextCancelsTheLoadingBatchAndRestartsIt(t *testing.T) {
	rig := newSearchRig(t, searchView(true, catItems...), map[string][]string{"ca": {"tiger"}, "cat": {"whale"}})
	rig.hold = true
	rig.session.Palette().Search("ca")
	rig.rest(1)
	if !rig.pendingNow() {
		t.Fatal("the batch is not loading for ca")
	}
	rig.session.Palette().Search("cat")
	if rig.pendingNow() {
		t.Fatal("typing a new text left the old batch loading")
	}
	rig.releaseHeld()
	if rig.pendingNow() {
		t.Fatal("the answer for the text left turned loading back on")
	}
	if got := rowTitles(rig.session.Palette().Search("cat")); len(got) != 1 || got[0] != "Cat" {
		t.Fatalf("rows for cat = %v, want the local match before its own answer", got)
	}
	rig.rest(1)
	if !rig.pendingNow() || len(rig.asked) != 2 || rig.asked[1] != "cat" {
		t.Fatalf("asked = %v, want cat asked after its pause and loading", rig.asked)
	}
	rig.releaseHeld()
	if got := rowTitles(rig.session.Palette().Search("cat")); len(got) != 2 || got[1] != "Whale" {
		t.Fatalf("rows for cat = %v, want Whale from the answer to cat", got)
	}
}
