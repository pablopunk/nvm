package palette

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
	"math"
	"testing"
)

func TestMediaGridCellHeightTracksPaneAndAspect(t *testing.T) {
	for _, tc := range []struct {
		width float32
		cols  int
		ratio float32
	}{{420, 3, 16.0 / 9}, {720, 3, 16.0 / 9}, {900, 3, 1}, {720, 4, 4.0 / 3}} {
		imageWidth := (tc.width-28-float32(tc.cols-1)*8)/float32(tc.cols) - 16
		got := mediaGridCellHeight(tc.width, tc.cols, tc.ratio)
		if math.Abs(float64((got-58)*tc.ratio-imageWidth)) > .01 {
			t.Errorf("%+v: height %v cannot fit image", tc, got)
		}
	}
}

func TestMediaGridResizeKeepsImageAndLabelInsideCell(t *testing.T) {
	for _, ratio := range []float32{16.0 / 9, 1, 4.0 / 3} {
		var ran []string
		view := rowsView("Grid", &ran, "A", "B", "C", "D")
		view.Grid = &Grid{Columns: 3, CellHeight: 180, ImageAspectRatio: ratio, Cell: func(c *ui.Context, item search.Item) {
			ui.Column(c).FillWidth().Children(func() {
				ui.Box(c).FillWidth().AspectRatio(ratio).Label("image-" + item.Title)
				ui.Text(c, "caption-"+item.Title).FontSize(13)
			})
		}}
		h := newSizedHarness(t, view, true, 420, 700)
		for _, width := range []int{420, 720, 900} {
			h.tt.SetSize(width, 700)
			h.tt.Frame()
			image, ok := h.tt.Find("image-A")
			if !ok {
				t.Fatal("image missing")
			}
			label, ok := h.tt.Find("caption-A")
			if !ok {
				t.Fatal("caption missing")
			}
			next, ok := h.tt.Find("image-D")
			if !ok {
				t.Fatal("next row missing")
			}
			if math.Abs(float64(image.W/image.H-ratio)) > .02 {
				t.Fatalf("width %d ratio %v: image %+v", width, ratio, image)
			}
			if label.Y+label.H > next.Y {
				t.Fatalf("width %d ratio %v: caption %+v overlaps next image %+v", width, ratio, label, next)
			}
		}
	}
}
