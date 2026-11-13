package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestDetailPaneOnlyChangesWhenTheListSettles(t *testing.T) {
	v := &View{Title: "Root", Providers: func() []search.Provider { return nil }}
	withDetail := &search.Item{Title: "File", Detail: func(*ui.Context, search.Host) {}}
	plain := &search.Item{Title: "Automate"}
	p := &Palette{}

	p.settled = false
	if p.showDetail(v, withDetail) {
		t.Fatal("a row that arrived while providers were answering opened the pane")
	}
	p.settled = true
	if !p.showDetail(v, withDetail) {
		t.Fatal("the settled list did not open the pane of its chosen row")
	}
	p.settled = false
	if !p.showDetail(v, withDetail) {
		t.Fatal("typing closed an open pane before the list settled")
	}
	if p.showDetail(v, plain) {
		t.Fatal("a row without a detail drew a pane")
	}
}
