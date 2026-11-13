package search

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestAKeystrokeWhoseProvidersAnswerInTimeShowsOneSnapshot checks that ranked providers that answer within
// FirstSnapshot produce a single, settled snapshot: no partial list for each provider that answers first.
func TestAKeystrokeWhoseProvidersAnswerInTimeShowsOneSnapshot(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true})
	a := fixedRows("a", Item{ID: "low", Kind: KindCommands, Title: "Low", Score: 10})
	b := fixedRows("b", Item{ID: "high", Kind: KindCommands, Title: "High", Score: 90})
	c.Query("wallpapers", []Provider{a, b})

	first := sink.next(t)
	if !first.Settled || !equalStrings(titles(first.Items), []string{"High", "Low"}) {
		t.Fatalf("first snapshot settled=%v rows=%v, want both providers ranked at once", first.Settled, titles(first.Items))
	}
	select {
	case extra := <-sink.ch:
		t.Fatalf("a second snapshot followed for the same keystroke: %v", titles(extra.Items))
	default:
	}
}

// TestRowsOnScreenKeepTheirOrderWhenALateProviderAnswers checks that a provider answering after the first snapshot
// adds its rows below the rows on screen, even when they rank higher, so nothing on screen moves.
func TestRowsOnScreenKeepTheirOrderWhenALateProviderAnswers(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot, Flat: true})
	release := make(chan struct{})
	late := gatedProvider{id: "late", first: Item{ID: "early", Kind: KindFiles, Title: "Early", Score: 20}, late: Item{ID: "best", Kind: KindFiles, Title: "Best", Score: 99}, release: release}
	fast := fixedRows("fast", Item{ID: "fast", Kind: KindCommands, Title: "Fast", Score: 50})
	c.Query("q", []Provider{late, fast})
	passFirstSnapshot(t, c, clock)

	first := sink.next(t)
	if !equalStrings(titles(first.Items), []string{"Fast", "Early"}) {
		t.Fatalf("first snapshot rows = %v", titles(first.Items))
	}
	close(release)
	snaps := sink.untilSettled(t, c.Gen())
	if got := titles(snaps[len(snaps)-1].Items); !equalStrings(got, []string{"Fast", "Early", "Best"}) {
		t.Fatalf("rows after the late answer = %v, want the late row below the rows on screen", got)
	}

	sink2 := newSnapshotSink()
	c.opts.OnSnapshot = sink2.OnSnapshot
	if warm, hit := c.Query("q", []Provider{fixedRows("late", Item{ID: "best", Kind: KindFiles, Title: "Best", Score: 99}), fast}); !hit || titles(warm.Items)[0] != "Best" {
		t.Fatalf("revisited query rows = %v, want the remembered result in its best order", titles(warm.Items))
	}
}
