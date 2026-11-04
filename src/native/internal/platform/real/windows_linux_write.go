package real

import (
	"context"
	"math"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/x11"
)

// fullScreenUnavailable is the copy for full screen, which needs the EWMH state message that xdotool cannot
// send. The command reports it honestly instead of claiming success.
const fullScreenUnavailable = "Full screen is not available on this desktop yet."

// SetFrame places a window's frame and reads back the frame the window reached. xdotool moves the frame's
// origin, but sizes the client, so only the size is converted with the window's decoration extents. The
// readback is converted back to a frame the same way.
func (x x11Windows) SetFrame(ctx context.Context, write platform.WindowWrite) (platform.WindowFrame, error) {
	window, err := x.ownedWindow(ctx, write.PID, write.Key)
	if err != nil {
		return platform.WindowFrame{}, err
	}
	frame := x11RectOf(write.Frame)
	if write.PositionOnly {
		frame.Width = window.Frame.Width
		frame.Height = window.Frame.Height
	}
	client := frame.Client(window.Extents)
	idArg := windowIDArg(window.ID)
	if _, err := x.call(ctx, "xdotool", "windowmove", "--sync", idArg, strconv.Itoa(frame.X), strconv.Itoa(frame.Y)); err != nil {
		return platform.WindowFrame{}, err
	}
	if !write.PositionOnly {
		if _, err := x.call(ctx, "xdotool", "windowsize", "--sync", idArg, strconv.Itoa(client.Width), strconv.Itoa(client.Height)); err != nil {
			return platform.WindowFrame{}, err
		}
	}
	reached, ok := x.readGeometry(ctx, window.ID)
	if !ok {
		return platform.WindowFrame{}, &platform.WindowError{Code: platform.WindowErrorTargetLost}
	}
	return platformRectOf(reached.Framed(window.Extents)), nil
}

// SetFullScreen is not available: EWMH full screen needs a client message that the installed tools cannot
// send, so the command reports the capability as unsupported rather than changing nothing silently.
func (x x11Windows) SetFullScreen(ctx context.Context, pid int, key string, _ bool) (platform.WindowFrame, error) {
	if _, err := x.ownedWindow(ctx, pid, key); err != nil {
		return platform.WindowFrame{}, err
	}
	return platform.WindowFrame{}, &platform.WindowError{Code: platform.WindowErrorUnsupported, Message: fullScreenUnavailable}
}

// Raise activates a window through the window manager, falling back to a plain raise when the manager does
// not answer activation requests.
func (x x11Windows) Raise(ctx context.Context, pid int, key string) error {
	window, err := x.ownedWindow(ctx, pid, key)
	if err != nil {
		return err
	}
	if _, err := x.call(ctx, "xdotool", "windowactivate", "--sync", windowIDArg(window.ID)); err == nil {
		return nil
	}
	_, err = x.call(ctx, "xdotool", "windowraise", windowIDArg(window.ID))
	return err
}

// ownedWindow returns a window that still exists and belongs to pid, with its current frame and extents.
func (x x11Windows) ownedWindow(ctx context.Context, pid int, key string) (x11Window, error) {
	id, ok := windowIDOf(key)
	if !ok {
		return x11Window{}, &platform.WindowError{Code: platform.WindowErrorTargetLost}
	}
	window, ok := x.readWindow(ctx, id)
	if !ok || window.PID != pid {
		return x11Window{}, &platform.WindowError{Code: platform.WindowErrorTargetLost}
	}
	return window, nil
}

// x11RectOf rounds a frame to whole X pixels.
func x11RectOf(frame platform.WindowFrame) x11.Rect {
	return x11.Rect{
		X:      int(math.Round(frame.X)),
		Y:      int(math.Round(frame.Y)),
		Width:  int(math.Round(frame.Width)),
		Height: int(math.Round(frame.Height)),
	}
}
