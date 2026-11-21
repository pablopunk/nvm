package design

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestSearchBackHasAnAccessibleNameAndReturnsItsEvent(t *testing.T) {
	clicks := 0
	tester := ui.NewTester(func(c *ui.Context) {
		if SearchRow(c, SearchData{Back: true, HideInput: true, Title: "Child"}).Back {
			clicks++
		}
	}, 480, 100)
	tester.Frame()
	if _, ok := tester.Find("Back"); !ok {
		t.Fatal("Back control has no accessible name")
	}
	if err := tester.Click("Back"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("Back events=%d, want one", clicks)
	}
}

func TestRootBrandIconIsNotNamedBack(t *testing.T) {
	tester := ui.NewTester(func(c *ui.Context) { SearchRow(c, SearchData{HideInput: true, Title: "Root"}) }, 480, 100)
	tester.Frame()
	if _, ok := tester.Find("Back"); ok {
		t.Fatal("root brand icon advertised a nonexistent Back action")
	}
}
