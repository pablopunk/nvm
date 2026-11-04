package ocr

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Timeouts bound one recognition, including the capture that comes before a screen read. The default matches the
// Electron helper; the bounds keep a caller from asking for a run that holds a worker for minutes.
const (
	DefaultTimeout = 30 * time.Second
	MinTimeout     = 5 * time.Second
	MaxTimeout     = 60 * time.Second
)

// Errors the service returns before any engine runs.
var (
	ErrNotImage     = errors.New("OCR supports image files only")
	ErrNotAbsolute  = errors.New("OCR needs an absolute file path")
	ErrEmptyRegion  = errors.New("a screen region needs a width and a height above zero")
	ErrNoTempFolder = errors.New("OCR needs a temporary folder for screen captures")
)

// Options configures one read. Languages are engine language codes or BCP 47 tags; empty lets the engine choose.
// Timeout is clamped to [MinTimeout, MaxTimeout]; zero means DefaultTimeout.
type Options struct {
	Languages []string
	Timeout   time.Duration
}

// Features says which reads the OS can perform. A read that is not available is not offered.
type Features struct {
	Image  bool
	Screen bool
}

// Availability reports the reads the capabilities allow: an image needs text.recognize, a screen read also needs
// screen.capture.
func Availability(caps platform.Capabilities) Features {
	recognize := caps.Supported(platform.CapabilityTextRecognize)
	return Features{Image: recognize, Screen: recognize && caps.Supported(platform.CapabilityScreenCapture)}
}

// Deps are the ports and folders a Service is built from. TempDir holds the screen captures while they are read.
type Deps struct {
	Text    platform.TextRecognizer
	Capture platform.ScreenCapturer
	FS      platform.FS
	TempDir string
}

// Service reads text from image files and from the screen. It holds no state between calls, so it is safe to use from
// several goroutines.
type Service struct {
	deps Deps
}

// New returns a service over the ports in deps.
func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// Image reads the text of the image at path, which must be absolute and have an image extension.
func (s *Service) Image(ctx context.Context, path string, opts Options) (platform.TextResult, error) {
	if !filepath.IsAbs(path) {
		return platform.TextResult{}, ErrNotAbsolute
	}
	if files.KindOf(path) != files.KindImage {
		return platform.TextResult{}, ErrNotImage
	}
	ctx, cancel := context.WithTimeout(ctx, boundedTimeout(opts.Timeout))
	defer cancel()
	return s.deps.Text.Recognize(ctx, path, opts.Languages)
}

// Screen reads the text the screen shows.
func (s *Service) Screen(ctx context.Context, opts Options) (platform.TextResult, error) {
	return s.screen(ctx, nil, opts)
}

// Region reads the text inside one rectangle of the screen, in screen coordinates.
func (s *Service) Region(ctx context.Context, region platform.Rect, opts Options) (platform.TextResult, error) {
	if region.Width <= 0 || region.Height <= 0 {
		return platform.TextResult{}, ErrEmptyRegion
	}
	return s.screen(ctx, &region, opts)
}

// screen captures the screen or a region to a new file, reads it and removes the file. The removal runs on every path,
// including a failed capture, so no screenshot outlives the call.
func (s *Service) screen(ctx context.Context, region *platform.Rect, opts Options) (platform.TextResult, error) {
	if s.deps.TempDir == "" {
		return platform.TextResult{}, ErrNoTempFolder
	}
	ctx, cancel := context.WithTimeout(ctx, boundedTimeout(opts.Timeout))
	defer cancel()
	name, err := captureName()
	if err != nil {
		return platform.TextResult{}, err
	}
	dst := filepath.Join(s.deps.TempDir, name)
	defer func() { _ = s.deps.FS.Remove(dst) }()
	if err := s.deps.Capture.Capture(ctx, dst, region); err != nil {
		return platform.TextResult{}, err
	}
	return s.deps.Text.Recognize(ctx, dst, opts.Languages)
}

// boundedTimeout returns timeout clamped to the service's bounds, or the default when it is zero.
func boundedTimeout(timeout time.Duration) time.Duration {
	switch {
	case timeout == 0:
		return DefaultTimeout
	case timeout < MinTimeout:
		return MinTimeout
	case timeout > MaxTimeout:
		return MaxTimeout
	default:
		return timeout
	}
}

// captureName returns a file name no other capture shares, so concurrent reads never read each other's screenshot.
func captureName() (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "nvm-ocr-" + hex.EncodeToString(random[:]) + ".png", nil
}
