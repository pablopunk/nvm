package apps

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Icon service limits. An icon larger than maxIconSide pixels, or a PNG that does not decode, is refused before
// it is decoded in full, so a hostile icon cannot exhaust memory.
const (
	iconCacheVersion   = "app-icon-v1"
	iconLoadTimeout    = 5 * time.Second
	defaultIconWorkers = 2
	maxIconSide        = 1024
	// iconRetryAfter keeps a failed icon from being queued again on every render.
	iconRetryAfter = time.Minute
)

var errIconTimeout = errors.New("apps: icon load timed out")

// IconOptions wires an Icons service. Shell, Files and Clock are required. PixelSize is the icon's edge in
// device pixels: the palette passes 32 DIPs times the scale of the display under the pointer (Screens port).
type IconOptions struct {
	Shell     platform.Shell
	Files     platform.FS
	Clock     platform.Clock
	Disk      IconStore // nil keeps icons in memory only
	PixelSize int
	Workers   int // concurrent loads; zero means defaultIconWorkers
	// Ready is called, off the caller's thread, after an icon for path has been loaded. The caller repaints.
	Ready func(path string)
	// Error reports a failed load, for logs. Nil discards it.
	Error func(path string, err error)
}

type iconEntry struct {
	mtime time.Time
	image image.Image
}

// Icons loads and caches app icons without blocking its caller. Get returns at once; a missing icon is queued on a
// pool of at most Workers goroutines, each decoding its PNG off the caller's thread. A path is queued once however
// often it is asked for, and each load is bounded by iconLoadTimeout on the Clock.
type Icons struct {
	opts IconOptions

	mu       sync.Mutex
	idle     *sync.Cond
	memory   map[string]iconEntry
	failedAt map[string]time.Time
	queued   []string
	queuedIn map[string]bool
	active   int
	loading  int
}

// NewIcons returns an icon service with an empty cache.
func NewIcons(opts IconOptions) *Icons {
	if opts.Workers <= 0 {
		opts.Workers = defaultIconWorkers
	}
	s := &Icons{opts: opts, memory: map[string]iconEntry{}, failedAt: map[string]time.Time{}, queuedIn: map[string]bool{}}
	s.idle = sync.NewCond(&s.mu)
	return s
}

// Get returns the icon of path when it is in memory, and nil otherwise. A missing icon is queued; Ready reports
// when it lands. Paths that are not applications or shortcuts are refused without queueing.
func (s *Icons) Get(path string) image.Image {
	if !IsIconPath(path) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.memory[path]; ok {
		return entry.image
	}
	if failed, ok := s.failedAt[path]; ok && s.opts.Clock.Now().Sub(failed) < iconRetryAfter {
		return nil
	}
	if !s.queuedIn[path] {
		s.queuedIn[path] = true
		s.queued = append(s.queued, path)
	}
	if s.active < s.opts.Workers && len(s.queued) > 0 {
		s.active++
		safego.Go("apps.icons", s.drain)
	}
	return nil
}

// IsIconPath reports whether path names something with an app icon: a macOS bundle or a Windows shortcut.
func IsIconPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".app") || strings.HasSuffix(lower, ".lnk")
}

// Pending reports how many icons are queued or loading.
func (s *Icons) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queued) + s.loading
}

// WaitIdle blocks until the queue is empty. Tests use it; the palette never waits.
func (s *Icons) WaitIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.queued) > 0 || s.loading > 0 {
		s.idle.Wait()
	}
}

// drain loads queued icons until the queue is empty, then lets its goroutine end.
func (s *Icons) drain(_ context.Context) {
	for {
		s.mu.Lock()
		if len(s.queued) == 0 {
			s.active--
			s.idle.Broadcast()
			s.mu.Unlock()
			return
		}
		path := s.queued[0]
		s.queued = s.queued[1:]
		delete(s.queuedIn, path)
		s.loading++
		s.mu.Unlock()

		img, err := s.load(path)

		s.mu.Lock()
		s.loading--
		if err == nil {
			s.memory[path] = img
			delete(s.failedAt, path)
		} else {
			s.failedAt[path] = s.opts.Clock.Now()
		}
		s.idle.Broadcast()
		s.mu.Unlock()
		if err != nil {
			s.reportError(path, err)
		} else if s.opts.Ready != nil {
			s.opts.Ready(path)
		}
	}
}

func (s *Icons) reportError(path string, err error) {
	if s.opts.Error != nil {
		s.opts.Error(path, err)
	}
}

// load produces the icon of path: from the disk cache when the key (path, modification time, size) matches,
// otherwise from the shell, decoded and stored.
func (s *Icons) load(path string) (iconEntry, error) {
	info, err := s.opts.Files.Stat(path)
	if err != nil {
		return iconEntry{}, err
	}
	mtime := info.ModTime()
	key := iconKey(path, mtime, s.opts.PixelSize)
	if s.opts.Disk != nil {
		if data, ok, readErr := s.opts.Disk.Read(key); readErr == nil && ok {
			if img, decodeErr := decodeIcon(data); decodeErr == nil {
				return iconEntry{mtime: mtime, image: img}, nil
			}
		}
	}
	data, err := s.fetch(path)
	if err != nil {
		return iconEntry{}, err
	}
	img, err := decodeIcon(data)
	if err != nil {
		return iconEntry{}, err
	}
	if s.opts.Disk != nil {
		_ = s.opts.Disk.Write(key, data)
	}
	return iconEntry{mtime: mtime, image: img}, nil
}

// fetch asks the shell for the PNG. The call cannot be cancelled, so after the timeout the worker moves on and the
// call finishes in its own goroutine; its result is dropped.
func (s *Icons) fetch(path string) ([]byte, error) {
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		data, err := s.opts.Shell.AppIcon(path, s.opts.PixelSize)
		done <- outcome{data, err}
	}()
	select {
	case out := <-done:
		return out.data, out.err
	case <-s.opts.Clock.After(iconLoadTimeout):
		return nil, errIconTimeout
	}
}

// decodeIcon checks the PNG header and dimensions before decoding, then decodes it.
func decodeIcon(data []byte) (image.Image, error) {
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("apps: icon is not a PNG: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxIconSide || config.Height > maxIconSide {
		return nil, fmt.Errorf("apps: icon is %dx%d, outside 1..%d", config.Width, config.Height, maxIconSide)
	}
	return png.Decode(bytes.NewReader(data))
}

// iconKey names an icon in the disk cache: the path, its modification time and the pixel size, hashed so the
// file name is safe and bounded.
func iconKey(path string, mtime time.Time, size int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%d", iconCacheVersion, path, mtime.UnixNano(), size)))
	return hex.EncodeToString(sum[:])
}
