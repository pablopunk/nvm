package supervisor

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// sameText recognizes every image as one fixed text.
type sameText struct{ text string }

func (r sameText) Recognize(context.Context, string, []string) (platform.TextResult, error) {
	return platform.TextResult{Text: r.text}, nil
}

// blankScreen captures a one-pixel image, or fails with err.
type blankScreen struct{ err error }

func (s blankScreen) Capture(_ context.Context, dst string, _ *platform.Rect) error {
	if s.err != nil {
		return s.err
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		return err
	}
	return os.WriteFile(dst, buffer.Bytes(), 0o600)
}

// indicatorLog records the passive indicators an extension shows.
type indicatorLog struct {
	mu    sync.Mutex
	shown []api.IndicatorInput
}

func (l *indicatorLog) ShowIndicator(_ string, input api.IndicatorInput) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shown = append(l.shown, input)
}

func (l *indicatorLog) HideIndicator(string, string) {}

func (l *indicatorLog) last() api.IndicatorInput {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.shown) == 0 {
		return api.IndicatorInput{}
	}
	return l.shown[len(l.shown)-1]
}

func ocrRuntime(t *testing.T, id, text string, screenErr error) (*Runtime, *fake.Clipboard, *indicatorLog) {
	t.Helper()
	clipboard := fake.NewClipboard()
	indicators := &indicatorLog{}
	r := installedRuntime(t, id, func(rig *servicesRig) {
		rig.deps.Capabilities = platform.NewCapabilities(platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)
		rig.deps.Clipboard = clipboard
		rig.deps.Indicators = indicators
		rig.deps.OCR = ocr.New(ocr.Deps{Text: sameText{text: text}, Capture: blankScreen{err: screenErr}, FS: rig.deps.Files, TempDir: t.TempDir()})
	})
	return r, clipboard, indicators
}

// TestInstalledOCRScreenCopiesVisibleTextAndReportsEachOutcome runs the installed OCR Screen extension: the recognized
// text lands on the clipboard with a line count, an empty screen says so, and a refused screen capture reports the
// missing screen recording permission instead of failing silently.
func TestInstalledOCRScreenCopiesVisibleTextAndReportsEachOutcome(t *testing.T) {
	requireUnixProcesses(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	invoke := api.Invocation{Ref: api.ActionRef{ID: "copy-visible-screen-text"}}

	r, clipboard, indicators := ocrRuntime(t, "local.ocr-screen", "first line\n\nsecond line\n", nil)
	runInstalledResult(t, ctx, r, invoke)
	if text, _ := clipboard.ReadText(); text != "first line\n\nsecond line" {
		t.Fatalf("clipboard = %q, want the recognized text", text)
	}
	if last := indicators.last(); last.Status != api.IndicatorSuccess || last.Subtitle != "Copied 2 lines to the clipboard" {
		t.Fatalf("indicator = %+v, want success naming two lines", last)
	}

	r, clipboard, indicators = ocrRuntime(t, "local.ocr-screen", "   ", nil)
	runInstalledResult(t, ctx, r, invoke)
	if text, _ := clipboard.ReadText(); text != "" {
		t.Fatalf("clipboard = %q, want it untouched for an empty screen", text)
	}
	if last := indicators.last(); last.Status != api.IndicatorError || last.Subtitle != "No visible text found" {
		t.Fatalf("indicator = %+v, want the no-text note", last)
	}

	r, _, indicators = ocrRuntime(t, "local.ocr-screen", "unused", errors.Join(platform.ErrDenied))
	if _, err := r.Call(ctx, "run", "", &invoke); err == nil || !strings.Contains(err.Error(), "screen record") {
		t.Fatalf("denied capture = %v, want a message about screen recording", err)
	}
	if last := indicators.last(); last.Status != api.IndicatorError || !strings.Contains(last.Subtitle, "screen record") {
		t.Fatalf("indicator = %+v, want the permission explained", last)
	}
}
