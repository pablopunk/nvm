//go:build linux

package real

import (
	"context"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/x11"
)

// Access reports allowed when the desktop answers EWMH queries. X11 has no per-app permission to grant.
func (x x11Windows) Access(ctx context.Context) (platform.WindowAccess, error) {
	if _, err := x.call(ctx, "xprop", "-root", "_NET_SUPPORTING_WM_CHECK"); err != nil {
		return platform.WindowAccessUnknown, nil
	}
	return platform.WindowAccessAllowed, nil
}

// Displays lists the monitors. Each work area is the monitor intersected with the desktop's _NET_WORKAREA,
// which keeps panels off the monitor they sit on when the panel spans the whole desktop. Without a published
// work area the whole monitor is used.
func (x x11Windows) Displays(ctx context.Context) ([]platform.WindowDisplay, error) {
	monitors, err := x.monitors(ctx)
	if err != nil {
		return nil, err
	}
	workArea, hasWorkArea, err := x.workArea(ctx)
	if err != nil {
		return nil, err
	}
	displays := make([]platform.WindowDisplay, 0, len(monitors))
	for index, monitor := range monitors {
		area := monitor.Bounds
		if overlap, ok := monitor.Bounds.Intersect(workArea); hasWorkArea && ok {
			area = overlap
		}
		displays = append(displays, platform.WindowDisplay{
			ID:       monitor.Name,
			Primary:  monitor.Primary || (index == 0 && !anyPrimary(monitors)),
			Frame:    platformRectOf(monitor.Bounds),
			WorkArea: platformRectOf(area),
		})
	}
	return displays, nil
}

func anyPrimary(monitors []x11.Monitor) bool {
	for _, monitor := range monitors {
		if monitor.Primary {
			return true
		}
	}
	return false
}

// RunningApps lists the apps that own a window, in front-to-back order.
func (x x11Windows) RunningApps(ctx context.Context) ([]platform.WindowApp, error) {
	records, err := x.AllWindows(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var apps []platform.WindowApp
	for _, record := range records {
		if seen[record.PID] {
			continue
		}
		seen[record.PID] = true
		apps = append(apps, platform.WindowApp{PID: record.PID, BundleID: record.BundleID, Name: record.AppName})
	}
	return apps, nil
}

// Windows lists the windows of one process, front to back.
func (x x11Windows) Windows(ctx context.Context, pid int) ([]platform.WindowRecord, error) {
	records, err := x.AllWindows(ctx)
	if err != nil {
		return nil, err
	}
	var matching []platform.WindowRecord
	for _, record := range records {
		if record.PID == pid {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

// FocusedWindow returns the top-most window of the process. X11 has no per-app focus, so this matches the
// Accessibility query it stands in for.
func (x x11Windows) FocusedWindow(ctx context.Context, pid int) (platform.WindowRecord, error) {
	windows, err := x.Windows(ctx, pid)
	if err != nil {
		return platform.WindowRecord{}, err
	}
	if len(windows) == 0 {
		return platform.WindowRecord{}, &platform.WindowError{Code: platform.WindowErrorNoWindow}
	}
	return windows[0], nil
}

// Frontmost returns the app that owns the active window.
func (x x11Windows) Frontmost(ctx context.Context) (platform.WindowApp, bool, error) {
	ids, err := x.activeWindow(ctx)
	if err != nil || len(ids) == 0 || ids[0] == 0 {
		return platform.WindowApp{}, false, err
	}
	window, ok := x.readWindow(ctx, ids[0])
	if !ok || window.PID == 0 {
		return platform.WindowApp{}, false, nil
	}
	return platform.WindowApp{PID: window.PID, BundleID: window.Class, Name: window.Class}, true, nil
}

// AllWindows lists every stacked window of every app that the user switches to, front to back, with ZIndex set.
// Nevermind's own windows, docks, panels and desktop windows are left out.
func (x x11Windows) AllWindows(ctx context.Context) ([]platform.WindowRecord, error) {
	ids, err := x.stackedWindows(ctx)
	if err != nil {
		return nil, err
	}
	monitors, err := x.monitors(ctx)
	if err != nil {
		return nil, err
	}
	var records []platform.WindowRecord
	for index := len(ids) - 1; index >= 0; index-- {
		window, ok := x.readWindow(ctx, ids[index])
		if !ok || window.PID == 0 || !window.isSwitchableWindow(os.Getpid()) {
			continue
		}
		records = append(records, window.record(monitors, len(ids)-1-index))
	}
	return records, nil
}
