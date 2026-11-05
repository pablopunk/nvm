package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Window wait bounds of the API: the default and the most a wait may last.
const (
	defaultWindowWait = 5 * time.Second
	maxWindowWait     = 30 * time.Second
	windowPollEvery   = 150 * time.Millisecond
)

// remoteWindowManagement lists displays and windows, places and focuses them, and waits for a window to appear. Each
// call is one exchange with the host's window engine, except the wait, which polls the listing.
type remoteWindowManagement struct {
	api.WindowManagement
	link *hostLink
}

func (w remoteWindowManagement) Displays() ([]api.DisplayInfo, error) {
	var displays []api.DisplayInfo
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpDisplays, nil, &displays)
	return displays, err
}

func (w remoteWindowManagement) Windows(query api.WindowQuery) ([]api.WindowInfo, error) {
	var windows []api.WindowInfo
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpWindows, query, &windows)
	return windows, err
}

func (w remoteWindowManagement) WaitForWindow(query api.WindowQuery, timeout time.Duration) (api.WindowInfo, bool, error) {
	deadline := time.Now().Add(clampWindowWait(timeout))
	for {
		windows, err := w.Windows(query)
		if err != nil {
			return api.WindowInfo{}, false, err
		}
		if len(windows) > 0 {
			return windows[0], true, nil
		}
		if !time.Now().Before(deadline) {
			return api.WindowInfo{}, false, nil
		}
		time.Sleep(windowPollEvery)
	}
}

func clampWindowWait(timeout time.Duration) time.Duration {
	switch {
	case timeout <= 0:
		return defaultWindowWait
	case timeout > maxWindowWait:
		return maxWindowWait
	}
	return timeout
}

func (w remoteWindowManagement) Focus(handle api.WindowHandle) (api.FocusResult, error) {
	var result api.FocusResult
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpFocus, protocol.FocusArgs{Handle: handle}, &result)
	return result, err
}

func (w remoteWindowManagement) FocusNeighbor(direction string, wrap bool) (api.FocusResult, error) {
	var result api.FocusResult
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpFocusNeighbor, protocol.FocusNeighborArgs{Direction: direction, Wrap: wrap}, &result)
	return result, err
}

func (w remoteWindowManagement) Apply(layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	var result api.ApplyResult
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpApply, protocol.ApplyArgs{Layout: layout, Options: options}, &result)
	return result, err
}

func (w remoteWindowManagement) ApplyTo(handle api.WindowHandle, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	var result api.ApplyResult
	err := w.link.Call(protocol.ServiceDesktopWindows, protocol.OpApplyTo, protocol.ApplyToArgs{Handle: handle, Layout: layout, Options: options}, &result)
	return result, err
}
