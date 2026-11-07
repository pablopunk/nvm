package design

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// galleryGoldens is the group of the gallery goldens, under testdata/golden/design-gallery.
var galleryGoldens = golden.Options{Group: "design-gallery", Width: GalleryWidth, Height: GalleryHeight}

// hoverRowInteraction moves the pointer onto the row titled "Hovered row", so that the row shows its hover plate.
func hoverRowInteraction(tt *ui.Tester) {
	if rect, ok := tt.Find("Hovered row"); ok {
		tt.Move(rect.X+8, rect.Y+4)
	}
}

// vibrancyInteraction shows the window with its system material, so the pane draws no fill of its own.
func vibrancyInteraction(tt *ui.Tester) {
	tt.SetVibrancy(true)
}

func TestGalleryFullWindow(t *testing.T) {
	golden.AssertMatrix(t, "gallery", Gallery, galleryGoldens)
}

func TestGalleryRowsPage(t *testing.T) {
	golden.AssertMatrix(t, "rows", galleryPage(galleryRows), galleryGoldens)
}

func TestGalleryRowsHovered(t *testing.T) {
	hovered := galleryGoldens
	hovered.Interact = hoverRowInteraction
	golden.AssertMatrix(t, "rows-hover", galleryPage(galleryRows), hovered)
}

func TestGalleryControlsPage(t *testing.T) {
	golden.AssertMatrix(t, "controls", galleryPage(galleryControls), galleryGoldens)
}

func TestGalleryStatesPage(t *testing.T) {
	golden.AssertMatrix(t, "states", galleryPage(galleryStates), galleryGoldens)
}

func TestGalleryStatesWithMaterial(t *testing.T) {
	material := galleryGoldens
	material.Interact = vibrancyInteraction
	golden.AssertMatrix(t, "states-material", galleryPage(galleryStates), material)
}

// rightToLeftInteraction lays the page out in the reading direction of Arabic and Hebrew.
func rightToLeftInteraction(tt *ui.Tester) {
	tt.SetDirection(ui.RightToLeft)
}

// doubleZoomInteraction renders the page at 200 percent text, the zoom a low-vision user asks for.
func doubleZoomInteraction(tt *ui.Tester) {
	tt.SetZoom(2)
}

func TestGalleryRowsRightToLeft(t *testing.T) {
	rtl := galleryGoldens
	rtl.Interact = rightToLeftInteraction
	golden.AssertMatrix(t, "rows-rtl", galleryPage(galleryRows), rtl)
}

func TestGalleryRowsAtDoubleZoom(t *testing.T) {
	zoomed := galleryGoldens
	zoomed.Interact = doubleZoomInteraction
	golden.AssertMatrix(t, "rows-zoom-200", galleryPage(galleryRows), zoomed)
}

// hoverMenuInteraction moves the pointer onto the menu row titled "Pin", so that the row shows the hover wash.
func hoverMenuInteraction(tt *ui.Tester) {
	if rect, ok := tt.Find("Pin"); ok {
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	}
}

func TestGalleryControlsMenuHovered(t *testing.T) {
	hovered := galleryGoldens
	hovered.Interact = hoverMenuInteraction
	golden.AssertMatrix(t, "controls-menu-hover", galleryPage(galleryControls), hovered)
}
