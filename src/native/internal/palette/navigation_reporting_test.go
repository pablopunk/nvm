package palette

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestNavigationReportingFollowsPresentedPages(t *testing.T) {
	var got []analytics.PageView
	root := &View{Title: "root"}
	h := newHarness(t, root, true, func(o *Options) { o.OnNavigation = func(v analytics.PageView) { got = append(got, v) } })
	want := []analytics.PageView{analytics.PageViewPalette}
	assertNavigation(t, got, want)
	h.frame()
	h.frame()
	assertNavigation(t, got, want)

	child := &View{Title: "extension"}
	h.p.Push(child)
	h.frame()
	want = append(want, analytics.PageViewExtension)
	assertNavigation(t, got, want)
	h.p.Pop()
	h.frame()
	want = append(want, analytics.PageViewPalette)
	assertNavigation(t, got, want)
	h.p.Hide()
	h.p.Show()
	h.frame()
	want = append(want, analytics.PageViewPalette)
	assertNavigation(t, got, want)
}

func TestNavigationReportingIgnoresQueryRefreshAndReplacementFrames(t *testing.T) {
	var got []analytics.PageView
	first := &View{Title: "first"}
	h := newHarness(t, &View{Title: "root"}, true, func(o *Options) { o.OnNavigation = func(v analytics.PageView) { got = append(got, v) } })
	h.p.Push(first)
	h.frame()
	h.p.SetQuery("canary query")
	h.frame()
	h.p.Refresh()
	h.frame()
	next := &View{Title: "replacement"}
	if !h.p.ReplaceIf(first, next) {
		t.Fatal("ReplaceIf rejected current view")
	}
	h.frame()
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewPalette, analytics.PageViewExtension})
}

func TestNavigationReportingSkipsActionLoadingAndReportsResolvedTarget(t *testing.T) {
	var got []analytics.PageView
	h := newHarness(t, &View{Title: "root"}, true, func(o *Options) { o.OnNavigation = func(v analytics.PageView) { got = append(got, v) } })
	var finish func() bool
	h.p.OpenAction("loading canary", search.Action{Dismiss: search.DismissNever, Run: func(host search.Host) { finish = host.(search.ActionLoadingHost).BeginAction() }})
	h.frame()
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewPalette})
	if !finish() {
		t.Fatal("loading completion rejected")
	}
	target := &View{Title: "resolved", Category: analytics.PageViewAIChat}
	h.p.Push(target)
	h.frame()
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewPalette, analytics.PageViewAIChat})
}

func TestNavigationReportingAllowsCachedRowsWhileLoading(t *testing.T) {
	var got []analytics.PageView
	view := &View{Title: "cached", Loading: func() bool { return true }, Pending: func() bool { return true }, Search: func(string) []search.Item { return []search.Item{{Title: "cached row"}} }}
	h := newHarness(t, view, true, func(o *Options) { o.OnNavigation = func(v analytics.PageView) { got = append(got, v) } })
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewPalette})
	if len(h.p.Results()) == 0 {
		t.Fatal("expected cached row to remain visible")
	}
}

func TestNavigationReportingRejectsUnknownCategories(t *testing.T) {
	var got []analytics.PageView
	newHarness(t, &View{Title: "root", Category: analytics.PageView("unknown")}, true, func(o *Options) { o.OnNavigation = func(v analytics.PageView) { got = append(got, v) } })
	assertNavigation(t, got, nil)
}

func TestNavigationReportingWaitsForFreshRevealAndDropsObsoleteFrames(t *testing.T) {
	var got []analytics.PageView
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true,
		OnNavigation: func(view analytics.PageView) { got = append(got, view) }})
	w := &opacityWindow{fakeWindow: newFakeWindow(), opacity: 1}
	p.Attach(w)
	tester := ui.NewTester(p.View, 720, 470)
	p.Show()
	tester.Frame()
	assertNavigation(t, got, nil)
	p.Hide()
	w.drain()
	assertNavigation(t, got, nil)
	p.Open(&View{Title: "superseded"})
	tester.Frame()
	p.Open(&View{Title: "target", Category: analytics.PageViewAIChat})
	tester.Frame()
	assertNavigation(t, got, nil)
	w.drain()
	tester.Frame()
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewAIChat})
}

func TestNavigationReportingRejectsNavigationDuringDrawing(t *testing.T) {
	var got []analytics.PageView
	var p *Palette
	moved := false
	root := &View{Title: "root", Body: func(_ *ui.Context, host search.Host) {
		if p.Visible() && !moved {
			moved = true
			host.Push(&View{Title: "target", Category: analytics.PageViewAIChat})
		}
	}}
	p = New(Options{Root: root, Clock: fake.NewClock(), Synchronous: true,
		OnNavigation: func(view analytics.PageView) { got = append(got, view) }})
	w := newFakeWindow()
	p.Attach(w)
	tester := ui.NewTester(p.View, 720, 470)
	p.Show()
	tester.Frame()
	w.drain()
	tester.Frame()
	assertNavigation(t, got, []analytics.PageView{analytics.PageViewAIChat})
}

func assertNavigation(t *testing.T, got, want []analytics.PageView) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("navigation reports = %v, want %v", got, want)
	}
}
