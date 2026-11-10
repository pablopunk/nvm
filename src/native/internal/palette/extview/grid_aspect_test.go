package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestGridAspectRatio(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  float32
	}{{"16/9", 16.0 / 9}, {"1/1", 1}, {"bad", 16.0 / 9}, {"1/0", 16.0 / 9}} {
		if got := gridAspectRatio(tc.input); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.input, got, tc.want)
		}
	}
}

func TestWideGridDefaultsToThreeColumnsAndRespectsExplicitCount(t *testing.T) {
	s := NewSession(Deps{}, "test", api.View{Kind: api.ViewGrid, Grid: &api.GridSpec{Layout: "wide"}})
	if got := s.gridColumns(); got != 3 {
		t.Fatalf("default wide columns = %d", got)
	}
	s.view.Grid.Columns = 4
	if got := s.gridColumns(); got != 4 {
		t.Fatalf("explicit columns = %d", got)
	}
	s.view.Grid.Layout = "compact"
	s.view.Grid.Columns = 0
	if got := s.gridColumns(); got != defaultGridColumns {
		t.Fatalf("compact default columns = %d", got)
	}
}
