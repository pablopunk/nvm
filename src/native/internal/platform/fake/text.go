package fake

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TextRecognizer answers Recognize from results keyed by image path and records every call. An unscripted path fails
// the test instead of guessing, as the fake helper does.
type TextRecognizer struct {
	mu      sync.Mutex
	results map[string]platform.TextResult
	errs    map[string]error
	calls   []TextCall
}

// TextCall is one recorded recognition.
type TextCall struct {
	Path      string
	Languages []string
}

var _ platform.TextRecognizer = (*TextRecognizer)(nil)

// NewTextRecognizer returns a recognizer with nothing scripted.
func NewTextRecognizer() *TextRecognizer {
	return &TextRecognizer{results: map[string]platform.TextResult{}, errs: map[string]error{}}
}

// Script answers recognition of path with result.
func (r *TextRecognizer) Script(path string, result platform.TextResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results[path] = result
}

// Fail makes recognition of path return err.
func (r *TextRecognizer) Fail(path string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs[path] = err
}

// Calls returns the recorded recognitions in order.
func (r *TextRecognizer) Calls() []TextCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// Recognize records the call, then returns what was scripted for path. A context that is already done fails first.
func (r *TextRecognizer) Recognize(ctx context.Context, path string, languages []string) (platform.TextResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, TextCall{Path: path, Languages: slices.Clone(languages)})
	result, ok := r.results[path]
	err := r.errs[path]
	r.mu.Unlock()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return platform.TextResult{}, ctxErr
	}
	if err != nil {
		return platform.TextResult{}, err
	}
	if !ok {
		return platform.TextResult{}, fmt.Errorf("fake text recognizer: no result scripted for %s", path)
	}
	return result, nil
}

// ScreenCapturer records the captures and fails them on request. It writes nothing; the file system is not involved.
type ScreenCapturer struct {
	mu       sync.Mutex
	err      error
	hook     func(dst string)
	captures []ScreenCapture
}

// ScreenCapture is one recorded capture.
type ScreenCapture struct {
	Dst    string
	Region *platform.Rect
}

var _ platform.ScreenCapturer = (*ScreenCapturer)(nil)

// NewScreenCapturer returns a capturer that succeeds.
func NewScreenCapturer() *ScreenCapturer {
	return &ScreenCapturer{}
}

// FailWith makes every capture return err; nil restores success.
func (c *ScreenCapturer) FailWith(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

// OnCapture runs hook with the destination of every capture, before the scripted error is returned. A test uses it to
// write the file a real capture would leave behind.
func (c *ScreenCapturer) OnCapture(hook func(dst string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hook = hook
}

// Captures returns the recorded captures in order.
func (c *ScreenCapturer) Captures() []ScreenCapture {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.captures)
}

// Capture records the capture and returns the scripted error, if any.
func (c *ScreenCapturer) Capture(ctx context.Context, dst string, region *platform.Rect) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var copied *platform.Rect
	if region != nil {
		value := *region
		copied = &value
	}
	c.captures = append(c.captures, ScreenCapture{Dst: dst, Region: copied})
	if c.hook != nil {
		c.hook(dst)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.err
}
