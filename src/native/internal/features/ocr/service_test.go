package ocr

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

var tempFolder = testpath.Abs("work", "tmp")

// deskWith returns a service over fakes, with the recognizer scripted to answer for every path it is asked about.
func deskWith(t *testing.T) (*Service, *fake.TextRecognizer, *fake.ScreenCapturer, *fake.FS) {
	t.Helper()
	text := fake.NewTextRecognizer()
	capture := fake.NewScreenCapturer()
	fsys := fake.NewFS()
	if err := fsys.MkdirAll(tempFolder, 0o700); err != nil {
		t.Fatal(err)
	}
	return New(Deps{Text: text, Capture: capture, FS: fsys, TempDir: tempFolder}), text, capture, fsys
}

func invoiceResult() platform.TextResult {
	return platform.TextResult{Text: "Invoice 42", Confidence: 0.9, Language: "en", Blocks: []platform.TextBlock{{Text: "Invoice 42", Confidence: 0.9}}}
}

func TestImageReadsAnImageFileThroughTheEngine(t *testing.T) {
	svc, text, _, _ := deskWith(t)
	photo := testpath.Abs("photos", "invoice.PNG")
	text.Script(photo, invoiceResult())
	got, err := svc.Image(context.Background(), photo, Options{Languages: []string{"en"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "Invoice 42" {
		t.Fatalf("text = %q", got.Text)
	}
	calls := text.Calls()
	if len(calls) != 1 || calls[0].Path != photo || calls[0].Languages[0] != "en" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestImageRefusesFilesThatAreNotImagesWithoutRunningAnEngine(t *testing.T) {
	svc, text, _, _ := deskWith(t)
	if _, err := svc.Image(context.Background(), testpath.Abs("docs", "report.pdf"), Options{}); !errors.Is(err, ErrNotImage) {
		t.Fatalf("err = %v, want ErrNotImage", err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("an engine ran for a refused file")
	}
}

func TestImageRefusesRelativePaths(t *testing.T) {
	svc, text, _, _ := deskWith(t)
	if _, err := svc.Image(context.Background(), "invoice.png", Options{}); !errors.Is(err, ErrNotAbsolute) {
		t.Fatalf("err = %v, want ErrNotAbsolute", err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("an engine ran for a relative path")
	}
}

func TestScreenCapturesToTheTempFolderReadsAndRemovesTheCapture(t *testing.T) {
	svc, _, capture, fsys := deskWith(t)
	var written string
	capture.OnCapture(func(dst string) {
		written = dst
		if err := fsys.WriteFile(dst, []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	svc.deps.Text = scriptedRecognizer{result: invoiceResult(), fsys: fsys, t: t}
	got, err := svc.Screen(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "Invoice 42" {
		t.Fatalf("text = %q", got.Text)
	}
	if !strings.HasPrefix(written, tempFolder+string(filepath.Separator)+"nvm-ocr-") || !strings.HasSuffix(written, ".png") {
		t.Fatalf("capture path = %q", written)
	}
	if _, err := fsys.Stat(written); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("capture still exists: %v", err)
	}
}

func TestRegionPassesTheRectangleToTheCapturer(t *testing.T) {
	svc, text, capture, _ := deskWith(t)
	text.Fail("ignored", errors.New("not reached"))
	region := platform.Rect{X: 10, Y: 20, Width: 300, Height: 120}
	_, _ = svc.Region(context.Background(), region, Options{})
	captures := capture.Captures()
	if len(captures) != 1 || captures[0].Region == nil || *captures[0].Region != region {
		t.Fatalf("captures = %+v", captures)
	}
}

func TestRegionRefusesEmptyRectanglesWithoutCapturing(t *testing.T) {
	svc, _, capture, _ := deskWith(t)
	for _, region := range []platform.Rect{{Width: 0, Height: 10}, {Width: 10, Height: -1}} {
		if _, err := svc.Region(context.Background(), region, Options{}); !errors.Is(err, ErrEmptyRegion) {
			t.Fatalf("region %+v err = %v, want ErrEmptyRegion", region, err)
		}
	}
	if len(capture.Captures()) != 0 {
		t.Fatal("a capture ran for an empty region")
	}
}

func TestFailedCaptureStillRemovesWhatItWroteAndReportsTheError(t *testing.T) {
	svc, text, capture, fsys := deskWith(t)
	denied := errors.New("screen recording refused")
	capture.OnCapture(func(dst string) { _ = fsys.WriteFile(dst, []byte("partial"), 0o600) })
	capture.FailWith(denied)
	if _, err := svc.Screen(context.Background(), Options{}); !errors.Is(err, denied) {
		t.Fatalf("err = %v, want the capture error", err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("the engine ran after a failed capture")
	}
	for _, name := range fsys.Paths() {
		if strings.HasSuffix(name, ".png") {
			t.Fatalf("capture left %s behind", name)
		}
	}
}

func TestScreenReadNeedsATempFolder(t *testing.T) {
	svc := New(Deps{Text: fake.NewTextRecognizer(), Capture: fake.NewScreenCapturer(), FS: fake.NewFS()})
	if _, err := svc.Screen(context.Background(), Options{}); !errors.Is(err, ErrNoTempFolder) {
		t.Fatalf("err = %v, want ErrNoTempFolder", err)
	}
}

func TestTimeoutIsClampedToTheServiceBounds(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		0:                DefaultTimeout,
		time.Second:      MinTimeout,
		10 * time.Second: 10 * time.Second,
		time.Hour:        MaxTimeout,
		MaxTimeout:       MaxTimeout,
		-time.Second:     MinTimeout,
		DefaultTimeout:   DefaultTimeout,
		MinTimeout / 2:   MinTimeout,
		MinTimeout:       MinTimeout,
		MaxTimeout + 1:   MaxTimeout,
		42 * time.Second: 42 * time.Second,
		59 * time.Second: 59 * time.Second,
		61 * time.Second: MaxTimeout,
		4 * time.Second:  MinTimeout,
	}
	for in, want := range cases {
		if got := boundedTimeout(in); got != want {
			t.Errorf("boundedTimeout(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestAvailabilityFollowsTheCapabilities(t *testing.T) {
	none := Availability(platform.Capabilities{})
	if none.Image || none.Screen {
		t.Fatalf("no capabilities = %+v", none)
	}
	textOnly := Availability(platform.NewCapabilities(platform.CapabilityTextRecognize))
	if !textOnly.Image || textOnly.Screen {
		t.Fatalf("text only = %+v, want images without screen reads", textOnly)
	}
	screenOnly := Availability(platform.NewCapabilities(platform.CapabilityScreenCapture))
	if screenOnly.Image || screenOnly.Screen {
		t.Fatalf("screen capture without a recognizer = %+v", screenOnly)
	}
	both := Availability(platform.NewCapabilities(platform.CapabilityTextRecognize, platform.CapabilityScreenCapture))
	if !both.Image || !both.Screen {
		t.Fatalf("both = %+v", both)
	}
}

func TestCaptureNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		name, err := captureName()
		if err != nil {
			t.Fatal(err)
		}
		if seen[name] {
			t.Fatalf("capture name %q repeated", name)
		}
		seen[name] = true
	}
}

// scriptedRecognizer answers the capture it is given and checks that the file exists when the engine reads it.
type scriptedRecognizer struct {
	result platform.TextResult
	fsys   *fake.FS
	t      *testing.T
}

func (r scriptedRecognizer) Recognize(_ context.Context, path string, _ []string) (platform.TextResult, error) {
	if _, err := r.fsys.Stat(path); err != nil {
		r.t.Fatalf("the engine read %s before it existed: %v", path, err)
	}
	return r.result, nil
}
