package extitems

import "testing"

func TestOffBarOnlyWhenTheSystemKeepsTheItemsOff(t *testing.T) {
	h := newHarnessFor(t, "darwin")
	if h.svc.OffBar("acme.menubar") {
		t.Fatal("an item on the bar counted as kept off")
	}
	if h.svc.OffBar("acme.unknown") {
		t.Fatal("an extension with no items counted as kept off")
	}

	h.item("main").SetOffBar(true)
	if !h.svc.OffBar("acme.menubar") {
		t.Fatal("an item the system keeps off the bar was not reported")
	}
}
