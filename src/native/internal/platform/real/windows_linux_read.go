//go:build linux

package real

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/x11"
)

// x11Window is one client window read from the desktop. Frame is the window's frame (the client rectangle
// grown by Extents), which is the rectangle the engine places and verifies.
type x11Window struct {
	ID         uint32
	PID        int
	Class      string
	Title      string
	Frame      x11.Rect
	Extents    x11.Extents
	FullScreen bool
	Minimized  bool
	Movable    bool
	Resizable  bool
}

// readWindow reads a window's properties and geometry. A window that vanished between listing and reading
// reports false, so callers skip it.
func (x x11Windows) readWindow(ctx context.Context, id uint32) (x11Window, bool) {
	properties, err := x.call(ctx, "xprop", "-id", windowIDArg(id), "_NET_WM_PID", "WM_CLASS", "_NET_WM_NAME", "WM_NAME", "_NET_WM_STATE", "_NET_WM_ALLOWED_ACTIONS", "_NET_FRAME_EXTENTS")
	if err != nil {
		return x11Window{}, false
	}
	client, ok := x.readGeometry(ctx, id)
	if !ok {
		return x11Window{}, false
	}
	parsed := x11.ParseProperties(properties)
	extents := parsed.Extents("_NET_FRAME_EXTENTS")
	pid, _ := parsed.Cardinal("_NET_WM_PID")
	title := parsed.String("_NET_WM_NAME")
	if title == "" {
		title = parsed.String("WM_NAME")
	}
	return x11Window{
		ID:         id,
		PID:        pid,
		Class:      parsed.WMClass(),
		Title:      title,
		Frame:      client.Framed(extents),
		Extents:    extents,
		FullScreen: parsed.HasAtom("_NET_WM_STATE", "_NET_WM_STATE_FULLSCREEN"),
		Minimized:  parsed.HasAtom("_NET_WM_STATE", "_NET_WM_STATE_HIDDEN"),
		Movable:    allowsAction(parsed, "_NET_WM_ACTION_MOVE"),
		Resizable:  allowsAction(parsed, "_NET_WM_ACTION_RESIZE"),
	}, true
}

// allowsAction reports whether a window may perform an action. A window that publishes no allowed-actions
// list is assumed to allow it, because the desktop cannot say otherwise.
func allowsAction(properties x11.Properties, action string) bool {
	if properties.Atoms("_NET_WM_ALLOWED_ACTIONS") == nil {
		return true
	}
	return properties.HasAtom("_NET_WM_ALLOWED_ACTIONS", action)
}

// readGeometry returns the client rectangle, as xwininfo reports it.
func (x x11Windows) readGeometry(ctx context.Context, id uint32) (x11.Rect, bool) {
	output, err := x.call(ctx, "xwininfo", "-id", windowIDArg(id))
	if err != nil {
		return x11.Rect{}, false
	}
	return x11.Geometry(output)
}

func (w x11Window) record(monitors []x11.Monitor, zIndex int) platform.WindowRecord {
	return platform.WindowRecord{
		Key:        windowKey(w.PID, w.ID),
		PID:        w.PID,
		BundleID:   w.Class,
		AppName:    w.Class,
		Title:      w.Title,
		Frame:      platformRectOf(w.Frame),
		DisplayID:  displayContaining(monitors, w.Frame),
		FullScreen: w.FullScreen,
		Minimized:  w.Minimized,
		Movable:    w.Movable,
		Resizable:  w.Resizable,
		ZIndex:     zIndex,
	}
}

// displayContaining names the monitor a window overlaps most, or "" when there are no monitors.
func displayContaining(monitors []x11.Monitor, frame x11.Rect) string {
	best, bestArea := "", 0
	for _, monitor := range monitors {
		overlap, ok := monitor.Bounds.Intersect(frame)
		if !ok {
			continue
		}
		if area := overlap.Width * overlap.Height; area > bestArea {
			best, bestArea = monitor.Name, area
		}
	}
	if best == "" && len(monitors) > 0 {
		return monitors[0].Name
	}
	return best
}

// stackedWindows lists the client windows bottom to top, as the desktop stacks them.
func (x x11Windows) stackedWindows(ctx context.Context) ([]uint32, error) {
	output, err := x.call(ctx, "xprop", "-root", "_NET_CLIENT_LIST_STACKING")
	if err != nil {
		return nil, err
	}
	return x11.WindowIDs(output), nil
}

func (x x11Windows) activeWindow(ctx context.Context) ([]uint32, error) {
	output, err := x.call(ctx, "xprop", "-root", "_NET_ACTIVE_WINDOW")
	if err != nil {
		return nil, err
	}
	return x11.WindowIDs(output), nil
}

// workArea returns the desktop's work area and whether the window manager publishes one.
func (x x11Windows) workArea(ctx context.Context) (x11.Rect, bool, error) {
	output, err := x.call(ctx, "xprop", "-root", "_NET_WORKAREA")
	if err != nil {
		return x11.Rect{}, false, err
	}
	area, ok := x11.WorkArea(output)
	return area, ok, nil
}

func (x x11Windows) monitors(ctx context.Context) ([]x11.Monitor, error) {
	output, err := x.call(ctx, "xrandr", "--query")
	if err != nil {
		return nil, err
	}
	monitors := x11.Monitors(output)
	if len(monitors) == 0 {
		return nil, &platform.WindowError{Code: platform.WindowErrorFailed, Message: "the X server reports no connected monitor"}
	}
	return monitors, nil
}

func windowKey(pid int, id uint32) string {
	return fmt.Sprintf("%d:%d", pid, id)
}

// windowIDOf reads the X window id back out of a key built by windowKey.
func windowIDOf(key string) (uint32, bool) {
	_, idText, found := strings.Cut(key, ":")
	if !found {
		return 0, false
	}
	id, err := strconv.ParseUint(idText, 10, 32)
	return uint32(id), err == nil
}

func windowIDArg(id uint32) string {
	return fmt.Sprintf("0x%x", id)
}

func platformRectOf(rect x11.Rect) platform.WindowFrame {
	return platform.WindowFrame{X: float64(rect.X), Y: float64(rect.Y), Width: float64(rect.Width), Height: float64(rect.Height)}
}
