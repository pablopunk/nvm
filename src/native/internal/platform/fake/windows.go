package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// WindowsCall is one recorded WindowManager call. Frame is the frame a write asked for.
type WindowsCall struct {
	Op    string
	PID   int
	Key   string
	Frame platform.WindowFrame
}

// WindowManager is a simulated desktop of windows and displays, the harness for engine tests. Writes
// behave like a real app: a minimum size is enforced, non-resizable windows keep their size, a refusing
// window keeps its frame, and readback returns what the window reached. Tests script the world with
// SetDisplays, AddWindow and Focus, then inspect what the code under test did through Calls.
type WindowManager struct {
	mu       sync.Mutex
	access   platform.WindowAccess
	displays []platform.WindowDisplay
	windows  map[string]*simWindow
	stack    []string
	failures map[string][]error
	calls    []WindowsCall
	nextID   int
}

// SimWindow describes a window to add. Zero values mean a resizable, movable, visible window with no
// minimum size that accepts writes. Key is generated as "<PID>:<n>" when empty.
type SimWindow struct {
	Key        string
	PID        int
	BundleID   string
	AppName    string
	Title      string
	Frame      platform.WindowFrame
	FullScreen bool
	Minimized  bool
	Fixed      bool
	MinWidth   float64
	MinHeight  float64
	Refuses    bool
}

type simWindow struct {
	SimWindow
}

var _ platform.WindowManager = (*WindowManager)(nil)

// NewWindowManager returns a window manager with access granted, no displays and no windows.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		access:   platform.WindowAccessAllowed,
		windows:  map[string]*simWindow{},
		failures: map[string][]error{},
	}
}

// SetAccess changes what Access reports.
func (w *WindowManager) SetAccess(access platform.WindowAccess) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.access = access
}

// SetDisplays replaces the monitors.
func (w *WindowManager) SetDisplays(displays ...platform.WindowDisplay) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.displays = append([]platform.WindowDisplay(nil), displays...)
}

// AddWindow places a window at the back of the stack and returns its key.
func (w *WindowManager) AddWindow(spec SimWindow) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if spec.Key == "" {
		w.nextID++
		spec.Key = fmt.Sprintf("%d:%d", spec.PID, w.nextID)
	}
	w.windows[spec.Key] = &simWindow{SimWindow: spec}
	w.stack = append(w.stack, spec.Key)
	return spec.Key
}

// Focus makes a window the front-most one, as if the user had clicked it.
func (w *WindowManager) Focus(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bringToFrontLocked(key)
}

// Window returns the current state of a window, for assertions.
func (w *WindowManager) Window(key string) (SimWindow, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.windows[key]
	if !ok {
		return SimWindow{}, false
	}
	return entry.SimWindow, true
}

// Remove closes a window, as if the user had closed it.
func (w *WindowManager) Remove(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.windows, key)
	w.stack = removeKey(w.stack, key)
}

// FailNext makes the next call of op return err. op is the method name, such as "SetFrame".
func (w *WindowManager) FailNext(op string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failures[op] = append(w.failures[op], err)
}

// Calls returns the recorded calls in order.
func (w *WindowManager) Calls() []WindowsCall {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]WindowsCall(nil), w.calls...)
}

// Access implements platform.WindowManager.
func (w *WindowManager) Access(ctx context.Context) (platform.WindowAccess, error) {
	return simCall(w, ctx, "Access", 0, "", platform.WindowFrame{}, func() (platform.WindowAccess, error) {
		return w.access, nil
	})
}

// Displays implements platform.WindowManager.
func (w *WindowManager) Displays(ctx context.Context) ([]platform.WindowDisplay, error) {
	return simCall(w, ctx, "Displays", 0, "", platform.WindowFrame{}, func() ([]platform.WindowDisplay, error) {
		return append([]platform.WindowDisplay(nil), w.displays...), nil
	})
}

// RunningApps implements platform.WindowManager. Apps are the owners of the windows.
func (w *WindowManager) RunningApps(ctx context.Context) ([]platform.WindowApp, error) {
	return simCall(w, ctx, "RunningApps", 0, "", platform.WindowFrame{}, func() ([]platform.WindowApp, error) {
		return w.appsLocked(), nil
	})
}

// Windows implements platform.WindowManager.
func (w *WindowManager) Windows(ctx context.Context, pid int) ([]platform.WindowRecord, error) {
	return simCall(w, ctx, "Windows", pid, "", platform.WindowFrame{}, func() ([]platform.WindowRecord, error) {
		return w.recordsLocked(func(entry *simWindow) bool { return entry.PID == pid }), nil
	})
}

// AllWindows implements platform.WindowManager.
func (w *WindowManager) AllWindows(ctx context.Context) ([]platform.WindowRecord, error) {
	return simCall(w, ctx, "AllWindows", 0, "", platform.WindowFrame{}, func() ([]platform.WindowRecord, error) {
		return w.recordsLocked(func(*simWindow) bool { return true }), nil
	})
}

// FocusedWindow implements platform.WindowManager. Only the front-most window can be focused, and only
// for its own process.
func (w *WindowManager) FocusedWindow(ctx context.Context, pid int) (platform.WindowRecord, error) {
	return simCall(w, ctx, "FocusedWindow", pid, "", platform.WindowFrame{}, func() (platform.WindowRecord, error) {
		front := w.frontLocked()
		if front == nil || front.PID != pid {
			return platform.WindowRecord{}, &platform.WindowError{Code: platform.WindowErrorNoWindow}
		}
		return w.recordLocked(front), nil
	})
}

// Frontmost implements platform.WindowManager.
func (w *WindowManager) Frontmost(ctx context.Context) (platform.WindowApp, bool, error) {
	type frontmostResult struct {
		app   platform.WindowApp
		found bool
	}
	result, err := simCall(w, ctx, "Frontmost", 0, "", platform.WindowFrame{}, func() (frontmostResult, error) {
		front := w.frontLocked()
		if front == nil {
			return frontmostResult{}, nil
		}
		app := platform.WindowApp{PID: front.PID, BundleID: front.BundleID, Name: front.AppName}
		return frontmostResult{app: app, found: true}, nil
	})
	return result.app, result.found, err
}

// SetFrame implements platform.WindowManager.
func (w *WindowManager) SetFrame(ctx context.Context, write platform.WindowWrite) (platform.WindowFrame, error) {
	return simCall(w, ctx, "SetFrame", write.PID, write.Key, write.Frame, func() (platform.WindowFrame, error) {
		entry, err := w.ownedLocked(write.PID, write.Key)
		if err != nil {
			return platform.WindowFrame{}, err
		}
		if !entry.Refuses {
			entry.Frame = placedFrame(entry, write)
		}
		return entry.Frame, nil
	})
}

// SetFullScreen implements platform.WindowManager. Entering full screen fills the window's display.
func (w *WindowManager) SetFullScreen(ctx context.Context, pid int, key string, fullScreen bool) (platform.WindowFrame, error) {
	return simCall(w, ctx, "SetFullScreen", pid, key, platform.WindowFrame{}, func() (platform.WindowFrame, error) {
		entry, err := w.ownedLocked(pid, key)
		if err != nil {
			return platform.WindowFrame{}, err
		}
		if !entry.Refuses {
			entry.FullScreen = fullScreen
			if fullScreen {
				entry.Frame = w.displayOfLocked(entry.Frame).Frame
			}
		}
		return entry.Frame, nil
	})
}

// Raise implements platform.WindowManager.
func (w *WindowManager) Raise(ctx context.Context, pid int, key string) error {
	_, err := simCall(w, ctx, "Raise", pid, key, platform.WindowFrame{}, func() (struct{}, error) {
		if _, err := w.ownedLocked(pid, key); err != nil {
			return struct{}{}, err
		}
		w.bringToFrontLocked(key)
		return struct{}{}, nil
	})
	return err
}

// simCall records one method call, then runs its body under the lock, unless a scripted failure is queued
// for op. A cancelled context fails first, as the real calls do.
func simCall[T any](w *WindowManager, ctx context.Context, op string, pid int, key string, frame platform.WindowFrame, body func() (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, WindowsCall{Op: op, PID: pid, Key: key, Frame: frame})
	if scripted := w.popFailureLocked(op); scripted != nil {
		return zero, scripted
	}
	return body()
}

func (w *WindowManager) popFailureLocked(op string) error {
	queued := w.failures[op]
	if len(queued) == 0 {
		return nil
	}
	w.failures[op] = queued[1:]
	return queued[0]
}
