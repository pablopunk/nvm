package golden

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	defaultWidth     = 320
	defaultHeight    = 240
	defaultTolerance = 0.001
	defaultThreshold = 4
)

// Options selects the window a view is rendered in and how strictly its frame is compared.
type Options struct {
	// Group names the directory under testdata/golden/ that holds the goldens. Empty means the name of
	// the test's package directory.
	Group string
	// Width and Height are the window's size in DIPs. Zero means 320 by 240.
	Width, Height int
	// Scale is the device pixels per DIP. Zero means 1.
	Scale float32
	// Dark renders the dark appearance instead of the light one.
	Dark bool
	// Tolerance is the largest fraction of differing pixels that still passes. Zero means 0.001.
	Tolerance float64
	// Threshold is the largest per-channel difference, out of 255, that does not make a pixel differ.
	// Zero means 4.
	Threshold uint8
	// Strict demands an exact match: zero tolerance and zero threshold. A one-pixel change fails.
	Strict bool
	// Advance moves the fake clock forward after the first frame, then renders again, so that a view that
	// animates (a reply still fading in, a list scrolling to its end) is compared in its settled state.
	Advance time.Duration
	// Interact drives the view after it settles, then renders again: it moves the pointer to a hover, sets
	// the window's vibrancy, or moves the focus. The frame it renders is compared like any other.
	Interact func(tt *ui.Tester)
}

// outcome reports what check did besides passing or failing.
type outcome struct {
	path    string
	updated bool
}

// Assert renders view in the window opts describes and compares the frame with the golden for name.
// With -update it writes the golden instead of comparing. It fails the test on a mismatch, a missing
// golden or a size mismatch, and writes the actual frame and a diff image under testdata/golden/_failures/.
func Assert(t *testing.T, name string, view func(c *ui.Context), opts Options) {
	t.Helper()
	result, err := check(name, view, withDefaults(opts), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	if result.updated {
		t.Logf("updated golden %s", result.path)
	}
}

// AssertMatrix runs Assert for the light and the dark appearance at 1x and 2x, as the subtests
// light-1x, light-2x, dark-1x and dark-2x. The Dark and Scale fields of opts are replaced.
func AssertMatrix(t *testing.T, name string, view func(c *ui.Context), opts Options) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			variant := opts
			variant.Dark = dark
			variant.Scale = scale
			t.Run(subtestName(dark, scale), func(t *testing.T) {
				Assert(t, name, view, variant)
			})
		}
	}
}

func subtestName(dark bool, scale float32) string {
	return fmt.Sprintf("%s-%gx", themeName(dark), scale)
}

func withDefaults(opts Options) Options {
	if opts.Group == "" {
		opts.Group = packageDirectoryName()
	}
	if opts.Width == 0 {
		opts.Width = defaultWidth
	}
	if opts.Height == 0 {
		opts.Height = defaultHeight
	}
	if opts.Scale == 0 {
		opts.Scale = 1
	}
	if opts.Strict {
		opts.Tolerance = 0
		opts.Threshold = 0
		return opts
	}
	if opts.Tolerance == 0 {
		opts.Tolerance = defaultTolerance
	}
	if opts.Threshold == 0 {
		opts.Threshold = defaultThreshold
	}
	return opts
}

// check renders view and compares it with the golden for name. testName is the test's name, for the hint
// that tells how to create a missing golden.
func check(name string, view func(c *ui.Context), opts Options, testName string) (outcome, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return outcome{}, fmt.Errorf("golden name %q must be a plain file name", name)
	}
	actual, err := storedFrame(render(view, opts))
	if err != nil {
		return outcome{}, err
	}
	path := goldenPath(opts, name)
	if *update {
		if err := writePNG(path, actual); err != nil {
			return outcome{}, err
		}
		return outcome{path: path, updated: true}, nil
	}
	golden, err := readPNG(path)
	if errors.Is(err, fs.ErrNotExist) {
		return outcome{path: path}, fmt.Errorf("golden %s does not exist; create it by running go test -run '^%s$' -update in the package directory", path, rootTestName(testName))
	}
	if err != nil {
		return outcome{path: path}, err
	}
	if golden.Bounds().Size() != actual.Bounds().Size() {
		written, werr := writeFailure(opts, name, actual, nil)
		return outcome{path: path}, joinErrors(
			fmt.Errorf("%s: frame is %v, golden is %v; actual written to %s", variantLabel(name, opts), actual.Bounds().Size(), golden.Bounds().Size(), written),
			werr)
	}
	diff, differing := diffPixels(golden, actual, opts.Threshold)
	total := actual.Bounds().Dx() * actual.Bounds().Dy()
	fraction := float64(differing) / float64(total)
	if fraction <= opts.Tolerance {
		return outcome{path: path}, nil
	}
	actualPath, werr := writeFailure(opts, name, actual, diff)
	return outcome{path: path}, joinErrors(
		fmt.Errorf("%s: %d of %d pixels differ (%.4f%%), above the %.4f%% tolerance; golden %s, actual %s, diff %s",
			variantLabel(name, opts), differing, total, fraction*100, opts.Tolerance*100, path, actualPath, diffPathFor(opts, name)),
		werr)
}

func render(view func(c *ui.Context), opts Options) *image.RGBA {
	tt := ui.NewTester(view, opts.Width, opts.Height)
	tt.SetScale(opts.Scale)
	tt.SetDark(opts.Dark)
	tt.Frame()
	if opts.Advance > 0 {
		tt.Advance(opts.Advance)
		tt.Frame()
	}
	if opts.Interact != nil {
		opts.Interact(tt)
		tt.Frame()
	}
	return tt.Image()
}

func variantLabel(name string, opts Options) string {
	return fmt.Sprintf("%s/%s %s", opts.Group, name, variantOf(opts))
}

// rootTestName returns the top-level test a subtest name belongs to, so that -run matches it.
func rootTestName(testName string) string {
	root, _, _ := strings.Cut(testName, "/")
	return root
}

func joinErrors(primary, secondary error) error {
	if secondary == nil {
		return primary
	}
	return errors.Join(primary, secondary)
}
