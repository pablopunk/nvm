package app

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestTypedExactTitleLeadsTheExtensionsList(t *testing.T) {
	rows := extensionListRows(&fakeExtensions{listings: []manager.Listing{goodListing()}}, "reload extensions")
	if len(rows) != 1 || rows[0].Title != "Reload Extensions" {
		t.Fatalf("rows for %q = %v, want only Reload Extensions", "reload extensions", rankedTitles(rows))
	}
}

func TestExtensionsListRanksMatchesByTitleAndKeepsEmptyQueryOrder(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}}
	if got, want := rankedTitles(extensionListRows(ctl, "")), []string{"Create Extension with AI", "Reload Extensions", "Good fixture"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("empty query rows = %v, want %v", got, want)
	}
	if got := rankedTitles(extensionListRows(ctl, "good")); len(got) == 0 || got[0] != "Good fixture" {
		t.Fatalf("rows for good = %v, want Good fixture first", got)
	}
}

func rankedTitles(rows []search.Item) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}
