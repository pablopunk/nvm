package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestResultControlIdentitySurvivesRetitlingAndReordering(t *testing.T) {
	rows := []search.Item{
		{ID: "automate", Kind: search.KindSearch, Title: "Automate hello"},
		{ID: "app", Kind: search.KindApplications, Title: "App"},
	}
	before := listRowKey(rows, 0)
	rows[0], rows[1] = rows[1], rows[0]
	rows[1].Title = "Automate hello there"
	if after := listRowKey(rows, 1); before != after {
		t.Fatalf("same result remounted after title/order change: %v -> %v", before, after)
	}
	other := []search.Item{{ID: "automate", Kind: search.KindApplications}}
	if listRowKey(other, 0) == before {
		t.Fatal("different result kinds shared a control identity")
	}
}

func TestLearnedResultControlIdentitySurvivesRetitling(t *testing.T) {
	rows := []search.Item{{Kind: search.KindSearch, LearnKey: "web-search", Title: "Search hello"}}
	before := listRowKey(rows, 0)
	rows[0].Title = "Search hello there"
	if after := listRowKey(rows, 0); before != after {
		t.Fatalf("same learned result remounted after title change: %v -> %v", before, after)
	}
	rows = append(rows, rows[0])
	if listRowKey(rows, 0) == listRowKey(rows, 1) {
		t.Fatal("anonymous rows sharing a learning key collided")
	}
}
