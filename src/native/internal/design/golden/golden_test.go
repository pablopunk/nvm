package golden

import (
	"errors"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

var (
	paper    = ui.Hex("#f5f2eb")
	inkBlack = ui.Hex("#101010")
)

// markerView draws a paper window and, when marked, one pixel at (10, 10) in mark.
func markerView(marked bool, mark ui.Color) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Box(c).Size(320, 240).Background(paper).Children(func() {
			if marked {
				ui.Box(c).Absolute().Left(10).Top(10).Size(1, 1).Background(mark)
			}
		})
	}
}

// harnessOptions is the window every harness test renders, in its own group.
var harnessOptions = Options{Group: "harness", Width: 320, Height: 240}

// useTempRoot points goldens at a temporary directory for the rest of the test.
func useTempRoot(t *testing.T) string {
	t.Helper()
	previous := goldenRoot
	goldenRoot = t.TempDir()
	t.Cleanup(func() { goldenRoot = previous })
	return goldenRoot
}

// setUpdate forces the update flag for the rest of the test, whatever go test was given.
func setUpdate(t *testing.T, value bool) {
	t.Helper()
	previous := *update
	*update = value
	t.Cleanup(func() { *update = previous })
}

func checkWith(t *testing.T, name string, view func(c *ui.Context), opts Options) (outcome, error) {
	t.Helper()
	return check(name, view, withDefaults(opts), t.Name())
}

func writeBaseline(t *testing.T) {
	t.Helper()
	setUpdate(t, true)
	if _, err := checkWith(t, "base", markerView(false, inkBlack), harnessOptions); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	setUpdate(t, false)
}

func TestIdenticalFramesPass(t *testing.T) {
	root := useTempRoot(t)
	writeBaseline(t)
	if _, err := checkWith(t, "base", markerView(false, inkBlack), harnessOptions); err != nil {
		t.Fatalf("identical frame failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, failuresDirectory)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a passing check wrote failure artifacts: %v", err)
	}
}

func TestOnePixelWithinTolerancePasses(t *testing.T) {
	useTempRoot(t)
	writeBaseline(t)
	if _, err := checkWith(t, "base", markerView(true, inkBlack), harnessOptions); err != nil {
		t.Fatalf("one differing pixel of 76800 failed the default tolerance: %v", err)
	}
}

func TestOnePixelOutsideToleranceFailsAndWritesDiff(t *testing.T) {
	root := useTempRoot(t)
	writeBaseline(t)
	strict := harnessOptions
	strict.Strict = true
	_, err := checkWith(t, "base", markerView(true, inkBlack), strict)
	if err == nil {
		t.Fatal("one differing pixel passed a strict comparison")
	}
	if !strings.Contains(err.Error(), "1 of 76800 pixels differ") {
		t.Fatalf("failure does not report the count: %v", err)
	}
	diff := readFailureImage(t, root, "base", "diff")
	if got := diff.RGBAAt(10, 10); got != differingColor {
		t.Fatalf("diff pixel at the change = %+v, want red", got)
	}
	if got := diff.RGBAAt(0, 0); got == differingColor {
		t.Fatal("diff marks an unchanged pixel as differing")
	}
	readFailureImage(t, root, "base", "actual")
}

func TestChannelChangesWithinThresholdAreIgnored(t *testing.T) {
	useTempRoot(t)
	writeBaseline(t)
	exactish := harnessOptions
	exactish.Tolerance = 1e-9  // any differing pixel fails; the threshold decides what differs
	small := ui.Hex("#f2f2eb") // 3 out of 255 on red, within the default threshold of 4
	large := ui.Hex("#f0f2eb") // 5 out of 255 on red, over it
	if _, err := checkWith(t, "base", markerView(true, small), exactish); err != nil {
		t.Fatalf("a 3/255 change is within the threshold: %v", err)
	}
	if _, err := checkWith(t, "base", markerView(true, large), exactish); err == nil {
		t.Fatal("a 5/255 change is over the threshold but passed")
	}
}

func TestMissingGoldenFailsWithUpdateHint(t *testing.T) {
	useTempRoot(t)
	setUpdate(t, false)
	_, err := checkWith(t, "absent", markerView(false, inkBlack), harnessOptions)
	if err == nil {
		t.Fatal("a missing golden passed")
	}
	for _, want := range []string{"does not exist", "-update", "absent.light.1x.png"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing golden error %q does not contain %q", err, want)
		}
	}
	if _, statErr := os.Stat(goldenPath(withDefaults(harnessOptions), "absent")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("a failed check created the golden: %v", statErr)
	}
}

func TestUpdateWritesGoldenThatThenPasses(t *testing.T) {
	useTempRoot(t)
	setUpdate(t, true)
	result, err := checkWith(t, "fresh", markerView(false, inkBlack), harnessOptions)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !result.updated {
		t.Fatal("update did not report writing the golden")
	}
	written, err := os.Open(result.path)
	if err != nil {
		t.Fatalf("golden not written: %v", err)
	}
	decoded, err := png.Decode(written)
	written.Close()
	if err != nil {
		t.Fatalf("golden is not a PNG: %v", err)
	}
	if size := decoded.Bounds().Size(); size.X != 320 || size.Y != 240 {
		t.Fatalf("golden is %v, want 320x240", size)
	}
	setUpdate(t, false)
	if _, err := checkWith(t, "fresh", markerView(false, inkBlack), harnessOptions); err != nil {
		t.Fatalf("freshly updated golden does not pass: %v", err)
	}
}

func TestSizeMismatchFailsAndWritesActual(t *testing.T) {
	root := useTempRoot(t)
	writeBaseline(t)
	narrow := harnessOptions
	narrow.Width = 200
	_, err := checkWith(t, "base", markerView(false, inkBlack), narrow)
	if err == nil || !strings.Contains(err.Error(), "frame is") {
		t.Fatalf("size mismatch error = %v", err)
	}
	readFailureImage(t, root, "base", "actual")
}

func TestNamesCannotEscapeTheGroup(t *testing.T) {
	useTempRoot(t)
	setUpdate(t, false)
	for _, name := range []string{"", "../escape", "nested/name", `back\slash`} {
		if _, err := checkWith(t, name, markerView(false, inkBlack), harnessOptions); err == nil {
			t.Errorf("golden name %q was accepted", name)
		}
	}
}

func TestDefaultsAndStrictMode(t *testing.T) {
	defaults := withDefaults(Options{})
	if defaults.Width != 320 || defaults.Height != 240 || defaults.Scale != 1 {
		t.Fatalf("window defaults = %+v", defaults)
	}
	if defaults.Tolerance != 0.001 || defaults.Threshold != 4 {
		t.Fatalf("comparison defaults = tolerance %v threshold %v", defaults.Tolerance, defaults.Threshold)
	}
	strict := withDefaults(Options{Strict: true, Tolerance: 0.5, Threshold: 9})
	if strict.Tolerance != 0 || strict.Threshold != 0 {
		t.Fatalf("strict did not zero tolerance and threshold: %+v", strict)
	}
}

func TestVariantNames(t *testing.T) {
	if got := variantOf(Options{Dark: true, Scale: 2}); got != "dark.2x" {
		t.Fatalf("variantOf = %q, want dark.2x", got)
	}
	if got := subtestName(false, 1); got != "light-1x" {
		t.Fatalf("subtestName = %q, want light-1x", got)
	}
	if got := rootTestName("TestA/light-1x"); got != "TestA" {
		t.Fatalf("rootTestName = %q, want TestA", got)
	}
}

func readFailureImage(t *testing.T, root, name, kind string) *image.RGBA {
	t.Helper()
	path := failurePath(withDefaults(harnessOptions), name, kind)
	if !strings.HasPrefix(path, root) {
		t.Fatalf("failure artifact %s is outside the temporary root %s", path, root)
	}
	img, err := readPNG(path)
	if err != nil {
		t.Fatalf("failure artifact %s: %v", kind, err)
	}
	return img
}
