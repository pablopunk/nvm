package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// rowIDs returns the IDs of the rows, in order.
func rowIDs(items []Item) []string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func scored(id string, score int) Item {
	return Item{ID: id, Kind: "Emoji", Score: score}
}

func TestAppendedRowsFollowTheRankedRowsInEmissionOrder(t *testing.T) {
	ranked := [][]Item{{scored("fire", 3), scored("flame", 1)}}
	appended := [][]Item{{scored("cat", 0), scored("dog", 0)}}
	if got := rowIDs(mergeAppended(ranked, appended, 0, true, false, true)); len(got) != 4 || got[0] != "fire" || got[1] != "flame" || got[2] != "cat" || got[3] != "dog" {
		t.Fatalf("rows = %v, want the ranked rows then the appended ones", got)
	}
}

func TestAppendedRowsNeverRankAboveTheRankedRows(t *testing.T) {
	ranked := [][]Item{{scored("low", 1)}}
	appended := [][]Item{{scored("high", 500)}}
	if got := rowIDs(mergeAppended(ranked, appended, 0, true, false, true)); got[0] != "low" || got[1] != "high" {
		t.Fatalf("rows = %v, want the appended row after the ranked one whatever its score", got)
	}
}

func TestAppendedRowAlreadyRankedIsDropped(t *testing.T) {
	ranked := [][]Item{{scored("fire", 3)}}
	appended := [][]Item{{scored("fire", 0), scored("cat", 0)}}
	if got := rowIDs(mergeAppended(ranked, appended, 0, true, false, true)); len(got) != 2 || got[1] != "cat" {
		t.Fatalf("rows = %v, want fire once and then cat", got)
	}
}

func TestAppendedRowsTakeTheirRoomFromTheLimitFirst(t *testing.T) {
	var ranked []Item
	for i := range 5 {
		ranked = append(ranked, scored(string(rune('a'+i)), 10-i))
	}
	appended := [][]Item{{scored("x", 0), scored("y", 0)}}
	got := rowIDs(mergeAppended([][]Item{ranked}, appended, 4, true, false, true))
	if len(got) != 4 || got[0] != "a" || got[1] != "b" || got[2] != "x" || got[3] != "y" {
		t.Fatalf("rows = %v, want the top two ranked rows and both appended rows within the limit", got)
	}
}

func TestAppendedRowsAloneFillTheirLimit(t *testing.T) {
	appended := [][]Item{{scored("x", 0), scored("y", 0), scored("z", 0)}}
	if got := rowIDs(mergeAppended(nil, appended, 2, true, false, true)); len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("rows = %v, want the first two appended rows", got)
	}
}

func TestCoordinatorKeepsTheAppendedBatchAfterTheRankedRows(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true})
	ranked := NewRowsProvider("local", func(context.Context, string) []Item {
		return []Item{scored("fire", 2)}
	})
	second := NewAppendedRowsProvider("meaning", func(context.Context, string) []Item {
		return []Item{scored("hot", 0), scored("fire", 0)}
	})
	snap, _ := c.Query("warm", []Provider{second, ranked})
	if got := rowIDs(snap.Items); len(got) != 2 || got[0] != "fire" || got[1] != "hot" {
		t.Fatalf("rows = %v, want the ranked fire row, then the appended hot row", got)
	}
}

func TestAppendedProviderIsDetectedOnlyWhenItSaysSo(t *testing.T) {
	plain := NewRowsProvider("plain", func(context.Context, string) []Item { return nil })
	if isAppended(plain) || !isAppended(NewAppendedRowsProvider("second", func(context.Context, string) []Item { return nil })) {
		t.Fatal("only NewAppendedRowsProvider rows are appended")
	}
}
