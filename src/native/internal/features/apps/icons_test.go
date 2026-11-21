package apps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Ported from app-icon-cache.test.ts (9 cases). The 1.x cache resolved a DOM data URL; the Go cache resolves an
// image.Image, and the PNG decode runs in the worker. The hook shell counts and gates the native calls.

// hookShell is a fake shell whose AppIcon runs a test hook: the hook decides the bytes or the error.
type hookShell struct {
	*fake.Shell
	hook func(path string, size int) ([]byte, error)
}

func (s hookShell) AppIcon(path string, size int) ([]byte, error) {
	return s.hook(path, size)
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 200, G: 120, B: 40, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type iconRig struct {
	icons   *Icons
	files   *fake.FS
	clock   *fake.Clock
	mu      sync.Mutex
	ready   []string
	errs    []error
	fetches atomic.Int32
}

func newIconRig(t *testing.T, workers int, disk IconStore, hook func(string, int) ([]byte, error)) *iconRig {
	t.Helper()
	rig := &iconRig{files: fake.NewFS(), clock: fake.NewClock()}
	shell := hookShell{Shell: fake.NewShell(), hook: func(path string, size int) ([]byte, error) {
		rig.fetches.Add(1)
		return hook(path, size)
	}}
	rig.icons = NewIcons(IconOptions{
		Shell: shell, Files: rig.files, Clock: rig.clock, Disk: disk, PixelSize: 64, Workers: workers,
		Ready: func(path string) {
			rig.mu.Lock()
			defer rig.mu.Unlock()
			rig.ready = append(rig.ready, path)
		},
		Error: func(_ string, err error) {
			rig.mu.Lock()
			defer rig.mu.Unlock()
			rig.errs = append(rig.errs, err)
		},
	})
	return rig
}

func (r *iconRig) readyCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ready)
}

func (r *iconRig) errorCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.errs)
}

// Ported: "app icon cache rejects unsupported paths without scheduling work".
func TestIconsRejectUnsupportedPathsWithoutQueueing(t *testing.T) {
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) { return testPNG(t, 2, 2), nil })
	if rig.icons.Get("/Applications/TextEdit.txt") != nil || rig.icons.Get("") != nil {
		t.Fatal("unsupported path returned an icon")
	}
	if rig.icons.Pending() != 0 {
		t.Fatalf("pending = %d, want 0", rig.icons.Pending())
	}
}

// Ported: "app icon cache dedupes concurrent requests and resolves waiters from one load". The disk write of the
// first load is checked too (ported from the written-key assertion).
func TestIconsDedupeConcurrentRequestsIntoOneLoad(t *testing.T) {
	disk := DiskIconStore{Files: fake.NewFS(), Dir: "/cache"}
	rig := newIconRig(t, 1, disk, func(string, int) ([]byte, error) { return testPNG(t, 2, 2), nil })
	rig.files.Seed("/Applications/TextEdit.app", []byte("bundle"))

	rig.icons.Get("/Applications/TextEdit.app")
	rig.icons.Get("/Applications/TextEdit.app")
	rig.icons.WaitIdle()

	if n := rig.fetches.Load(); n != 1 {
		t.Fatalf("native loads = %d, want 1", n)
	}
	if rig.readyCount() != 1 {
		t.Fatalf("ready callbacks = %d, want 1", rig.readyCount())
	}
	if img := rig.icons.Get("/Applications/TextEdit.app"); img == nil {
		t.Fatal("icon not in memory after its load")
	}
	info, _ := rig.files.Stat("/Applications/TextEdit.app")
	if _, ok, _ := disk.Read(iconKey("/Applications/TextEdit.app", info.ModTime(), 64)); !ok {
		t.Fatal("loaded icon was not written to the disk cache under its key")
	}
}

// Ported: "app icon cache loads Windows Start Menu shortcut icons".
func TestIconsLoadWindowsShortcutIcons(t *testing.T) {
	const shortcut = `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Example.LNK`
	rig := newIconRig(t, 1, nil, func(path string, _ int) ([]byte, error) {
		if path != shortcut {
			return nil, errors.New("unexpected path " + path)
		}
		return testPNG(t, 4, 4), nil
	})
	rig.files.Seed(shortcut, []byte("lnk"))
	if rig.icons.Get(shortcut) != nil {
		t.Fatal("icon returned before it loaded")
	}
	rig.icons.WaitIdle()
	if rig.readyCount() != 1 || rig.icons.Get(shortcut) == nil {
		t.Fatalf("shortcut icon did not load: ready=%d", rig.readyCount())
	}
}

// Ported: "app icon cache uses disk cache before loading native app icon".
func TestIconsUseDiskCacheBeforeNativeLoad(t *testing.T) {
	disk := DiskIconStore{Files: fake.NewFS(), Dir: "/cache"}
	rig := newIconRig(t, 1, disk, func(string, int) ([]byte, error) {
		t.Error("native load ran although the disk cache held the icon")
		return nil, errors.New("unexpected")
	})
	rig.files.Seed("/Applications/TextEdit.app", []byte("bundle"))
	info, _ := rig.files.Stat("/Applications/TextEdit.app")
	if err := disk.Write(iconKey("/Applications/TextEdit.app", info.ModTime(), 64), testPNG(t, 3, 3)); err != nil {
		t.Fatal(err)
	}
	rig.icons.Get("/Applications/TextEdit.app")
	rig.icons.WaitIdle()
	if n := rig.fetches.Load(); n != 0 {
		t.Fatalf("native loads = %d, want 0", n)
	}
	if rig.readyCount() != 1 {
		t.Fatalf("ready callbacks = %d, want 1", rig.readyCount())
	}
}

// Ported: "app icon cache schedules backlog batches when pending work remains". With one worker the loads run one
// at a time: the gate lets exactly one native call finish per release, and the largest number seen in flight is 1.
func TestIconsBacklogRunsWithinWorkerBound(t *testing.T) {
	gate := make(chan struct{})
	var inFlight, peak atomic.Int32
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) {
		now := inFlight.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		<-gate
		inFlight.Add(-1)
		return testPNG(t, 2, 2), nil
	})
	for _, name := range []string{"App1", "App2", "App3"} {
		rig.files.Seed("/Applications/"+name+".app", []byte("bundle"))
		rig.icons.Get("/Applications/" + name + ".app")
	}
	if pending := rig.icons.Pending(); pending != 3 {
		t.Fatalf("pending after three requests = %d, want 3", pending)
	}
	for range 3 {
		gate <- struct{}{}
	}
	rig.icons.WaitIdle()
	if peak.Load() != 1 {
		t.Fatalf("peak concurrent loads = %d, want 1", peak.Load())
	}
	if rig.readyCount() != 3 || rig.icons.Pending() != 0 {
		t.Fatalf("ready = %d, pending = %d; want 3 and 0", rig.readyCount(), rig.icons.Pending())
	}
}

// Ported: "withTimeout resolves with value when promise settles first".
func TestIconsFetchReturnsBeforeTimeout(t *testing.T) {
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) { return testPNG(t, 2, 2), nil })
	rig.files.Seed("/Applications/Quick.app", []byte("bundle"))
	rig.icons.Get("/Applications/Quick.app")
	rig.icons.WaitIdle()
	if rig.readyCount() != 1 || rig.errorCount() != 0 {
		t.Fatalf("ready = %d, errors = %d; want 1 and 0", rig.readyCount(), rig.errorCount())
	}
}

// Ported: "withTimeout resolves with fallback when promise times out". The 1.x test was skipped because an unref'd
// timer cannot keep the event loop alive; the fake clock makes it deterministic, so it runs here.
func TestIconsTimeOutAHungNativeLoad(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) {
		<-release
		return testPNG(t, 2, 2), nil
	})
	rig.files.Seed("/Applications/Hung.app", []byte("bundle"))
	rig.icons.Get("/Applications/Hung.app")
	rig.clock.AwaitTimers(1)
	rig.clock.Advance(iconLoadTimeout)
	rig.icons.WaitIdle()
	if rig.readyCount() != 0 {
		t.Fatal("a timed-out load delivered an icon")
	}
	if rig.errorCount() != 1 || !errors.Is(rig.errs[0], errIconTimeout) {
		t.Fatalf("errors = %v, want one timeout", rig.errs)
	}
}

// Ported: "withTimeout rejects when promise rejects (fallback is only for timeout)".
func TestIconsReportNativeErrorsAsErrorsNotTimeouts(t *testing.T) {
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) { return nil, errors.New("fail") })
	rig.files.Seed("/Applications/Broken.app", []byte("bundle"))
	rig.icons.Get("/Applications/Broken.app")
	rig.icons.WaitIdle()
	if rig.errorCount() != 1 || rig.errs[0].Error() != "fail" {
		t.Fatalf("errors = %v, want [fail]", rig.errs)
	}
}

// Ported: "app icon cache returns null for individual load failures (rejection, not timeout)". The failed path is
// not queued again on the next request inside the retry window, so a broken icon costs one load.
func TestIconsFailedLoadReturnsNilAndIsNotRequeuedAtOnce(t *testing.T) {
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) { return nil, errors.New("injected failure") })
	rig.files.Seed("/Applications/Broken.app", []byte("bundle"))
	if rig.icons.Get("/Applications/Broken.app") != nil {
		t.Fatal("icon returned before the load")
	}
	rig.icons.WaitIdle()
	if rig.icons.Get("/Applications/Broken.app") != nil {
		t.Fatal("failed icon returned an image")
	}
	if rig.icons.Pending() != 0 || rig.fetches.Load() != 1 {
		t.Fatalf("pending = %d, loads = %d; want 0 and 1", rig.icons.Pending(), rig.fetches.Load())
	}
	rig.clock.Advance(iconRetryAfter)
	rig.icons.Get("/Applications/Broken.app")
	rig.icons.WaitIdle()
	if rig.fetches.Load() != 2 {
		t.Fatalf("loads after the retry window = %d, want 2", rig.fetches.Load())
	}
}

// Hostile input: an icon that is not a PNG, or whose declared size is absurd, is refused before a full decode.
func TestDecodeIconRefusesNonPNGAndOversizedImages(t *testing.T) {
	if _, err := decodeIcon([]byte("not a png at all")); err == nil {
		t.Error("non-PNG accepted")
	}
	huge := testPNG(t, 2, 2)
	binary.BigEndian.PutUint32(huge[16:20], maxIconSide+1)
	if _, err := decodeIcon(huge); err == nil {
		t.Error("PNG wider than the limit accepted")
	}
	if _, err := decodeIcon(testPNG(t, 8, 8)); err != nil {
		t.Errorf("valid PNG refused: %v", err)
	}
}

// The caller never waits for the native load: Get returns before the hook has finished.
func TestIconsGetDoesNotWaitForNativeLoad(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	rig := newIconRig(t, 1, nil, func(string, int) ([]byte, error) {
		close(started)
		<-release
		return testPNG(t, 2, 2), nil
	})
	rig.files.Seed("/Applications/Slow.app", []byte("bundle"))
	if img := rig.icons.Get("/Applications/Slow.app"); img != nil {
		t.Fatal("Get returned an image before the load")
	}
	<-started
	close(release)
	rig.icons.WaitIdle()
	if rig.readyCount() != 1 {
		t.Fatalf("ready callbacks = %d, want 1", rig.readyCount())
	}
}
