package extitems

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const mainBarItemID = "ext:acme.menubar:main"

// On Linux the title joins the tooltip, so a ticking title changes the tooltip each render. The item keeps its place:
// its hover text changes in place instead of the item being closed and created again on every tick.
func TestTickingTitleOnLinuxUpdatesTheTooltipInPlace(t *testing.T) {
	h := newHarnessFor(t, "linux")
	first := h.item("main")
	h.source.set(func(s *fakeSource) { s.bars = []api.MenuBar{clockBar("tick 02")} })
	h.advance(time.Second)

	if got := h.item("main"); got != first {
		t.Fatal("the item was recreated when its title ticked")
	}
	if created := countTraysWithID(h.tray.Trays(), mainBarItemID); created != 1 {
		t.Fatalf("items created for the bar = %d, want 1", created)
	}
	if want, got := mainTooltipFor("tick 02"), first.ToolTip(); got != want {
		t.Fatalf("hover text = %q, want %q", got, want)
	}
}

func mainTooltipFor(title string) string {
	_, tooltip := menubar.StatusText("linux", title, "Clock")
	return tooltip
}

func countTraysWithID(trays []*fake.Tray, id string) int {
	count := 0
	for _, tray := range trays {
		if tray.Options().ID == id {
			count++
		}
	}
	return count
}
