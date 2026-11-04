//go:build rig

package rigtest

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

// The WP4.3 journey against the real app under the rig: the camera fixture asks the page for the camera, and the box
// has no camera, so the page's own start fails and the view shows the reason with the status "Camera unavailable".
// The webview fixture opens its page in the system web view without the palette's unavailable notice, and the page's
// heading paints under the header. The page is a native surface and not on the palette's accessibility tree, so the
// webview is checked by the desktop capture, and the camera by the text the palette draws around its page.

const wp43MediaFixture = "acme.media"

// TestJourneyWP43CameraFailsWithAReasonAndWebviewOpens enables the media fixture, opens the camera view and checks the
// failed start is explained, then opens the webview view and checks it is shown rather than refused.
func TestJourneyWP43CameraFailsWithAReasonAndWebviewOpens(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, wp43MediaFixture)
	app := Launch(t, Options{Home: home, Run: "first"})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "media", "Media fixture")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Camera fixture"))
	openRootFixtureRow(t, app, "Camera fixture")
	must(t, app.Key("Return"))
	_, err := app.WaitFor("Camera Issue", elementWait)
	must(t, err)
	_, err = app.WaitFor("Camera unavailable", elementWait)
	must(t, err)
	shoot(t, app, "wp43-1-camera-failed")
	must(t, app.Key("Escape"))
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Webview fixture"))
	openRootFixtureRow(t, app, "Webview fixture")
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Webview fixture", elementWait)
	must(t, err)
	tree, err := app.Tree()
	must(t, err)
	if strings.Contains(fmt.Sprintf("%+v", tree), "system web view, which this system does not have") {
		t.Fatal("the webview fixture showed the unavailable notice on a system whose web view is installed")
	}
	waitForPageInk(t, app, "wp43-2-webview-painted")
	shoot(t, app, "wp43-2-webview-open")
	hidePalette(t, app)
}

// waitForPageInk polls the desktop until the web page has painted dark text under the palette's header. The page is a
// native surface that the palette-only capture leaves out, so the desktop capture is the evidence the rig has of it.
func waitForPageInk(t *testing.T, app *App, name string) {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	for {
		path, err := app.Screenshot(name)
		must(t, err)
		palette, err := app.PaletteWindow()
		must(t, err)
		painted, err := hasDarkInkUnderHeader(path, palette.Bounds)
		must(t, err)
		if painted {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the web page painted no text under the header; desktop %s", path)
		}
		time.Sleep(pollInterval)
	}
}

// hasDarkInkUnderHeader reports whether the desktop capture at path holds an opaque dark pixel in the band inside the
// palette where a page's first lines draw, under its header. A transparent pixel is not ink.
func hasDarkInkUnderHeader(path string, palette Rect) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return false, err
	}
	const headerBottom, pageBandBottom, inset = 58, 160, 16
	bounds := img.Bounds()
	x0, x1 := max(int(palette.X)+inset, bounds.Min.X), min(int(palette.X+palette.Width)-inset, bounds.Max.X)
	y0, y1 := max(int(palette.Y)+headerBottom, bounds.Min.Y), min(int(palette.Y)+pageBandBottom, bounds.Max.Y)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0xffff && r < 0x7800 && g < 0x7800 && b < 0x7800 {
				return true, nil
			}
		}
	}
	return false, nil
}
