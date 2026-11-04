package real

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/swifthelper"
)

// macosHelperName is the window helper binary that ships in the app's resources, beside the executable otherwise.
const macosHelperName = "macos-selected-text"

// macosCallTimeout bounds one helper call, as the Electron window manager does.
const macosCallTimeout = 5 * time.Second

// helperUnreadable is the copy when the helper's answer is not an envelope.
const helperUnreadable = "The window helper returned an unreadable response."

// macosHelperPath returns the helper bundled with the app, else the one beside the executable.
func macosHelperPath() string {
	executableDir := ""
	if executable, err := os.Executable(); err == nil {
		executableDir = filepath.Dir(executable)
	}
	return chooseMacosHelperPath(existingBundledResource(macosHelperName), executableDir)
}

// chooseMacosHelperPath prefers the bundled helper, then the one in the executable's directory, then the bare name.
func chooseMacosHelperPath(bundled, executableDir string) string {
	switch {
	case bundled != "":
		return bundled
	case executableDir != "":
		return filepath.Join(executableDir, macosHelperName)
	}
	return macosHelperName
}

// macHelper runs the Swift window helper, a child process with the command line the Electron app uses. It is the
// only place that speaks the helper's process contract; the window and permission ports call it.
type macHelper struct {
	helper platform.Helper
	binary string
}

func newMacHelper(helper platform.Helper) macHelper {
	return macHelper{helper: helper, binary: macosHelperPath()}
}

// run calls the helper's window subcommand and returns its envelope. A process that fails to start, or prints
// nothing readable, is a failed call.
func (m macHelper) run(ctx context.Context, args ...string) (swifthelper.Envelope, error) {
	callCtx, cancel := context.WithTimeout(ctx, macosCallTimeout)
	defer cancel()
	result, err := m.helper.Run(callCtx, m.binary, append([]string{swifthelper.Subcommand}, args...), nil)
	if err != nil {
		return swifthelper.Envelope{}, &platform.WindowError{Code: platform.WindowErrorFailed, Message: err.Error()}
	}
	envelope, parseErr := swifthelper.ParseEnvelope(result.Stdout)
	if parseErr != nil {
		return swifthelper.Envelope{}, &platform.WindowError{Code: platform.WindowErrorFailed, Message: helperUnreadable}
	}
	return envelope, nil
}

// call runs a helper call whose data decodes into T.
func call[T any](ctx context.Context, m macHelper, args ...string) (T, error) {
	var zero T
	envelope, err := m.run(ctx, args...)
	if err != nil {
		return zero, err
	}
	if err := envelope.Err(); err != nil {
		return zero, err
	}
	value, err := swifthelper.Decode[T](envelope)
	if err != nil {
		return zero, &platform.WindowError{Code: platform.WindowErrorFailed, Message: helperUnreadable}
	}
	return value, nil
}

// macWindows drives windows through the Accessibility helper. L5 (a macOS runner with Accessibility granted) has
// not verified this backend yet.
type macWindows struct {
	macHelper
}

// Access asks the helper for its permission state. Anything but a clear answer is unknown.
func (m macWindows) Access(ctx context.Context) (platform.WindowAccess, error) {
	envelope, err := m.run(ctx, "permissions")
	switch {
	case err != nil:
		return platform.WindowAccessUnknown, nil
	case envelope.Code == swifthelper.CodeOK:
		return platform.WindowAccessAllowed, nil
	case envelope.Code == swifthelper.CodeDenied:
		return platform.WindowAccessDenied, nil
	}
	return platform.WindowAccessUnknown, nil
}

func (m macWindows) Displays(ctx context.Context) ([]platform.WindowDisplay, error) {
	displays, err := call[[]swifthelper.Display](ctx, m.macHelper, "displays")
	if err != nil {
		return nil, err
	}
	converted := make([]platform.WindowDisplay, 0, len(displays))
	for _, display := range displays {
		converted = append(converted, platform.WindowDisplay{
			ID:       display.ID,
			Primary:  display.IsPrimary,
			Frame:    display.Frame.Platform(),
			WorkArea: display.WorkArea.Platform(),
		})
	}
	return converted, nil
}

func (m macWindows) RunningApps(ctx context.Context) ([]platform.WindowApp, error) {
	apps, err := call[[]swifthelper.App](ctx, m.macHelper, "apps")
	if err != nil {
		return nil, err
	}
	converted := make([]platform.WindowApp, 0, len(apps))
	for _, app := range apps {
		converted = append(converted, platform.WindowApp{PID: app.PID, BundleID: app.BundleID, Name: app.Name})
	}
	return converted, nil
}

// Windows lists one process's windows from the stacking listing, front to back.
func (m macWindows) Windows(ctx context.Context, pid int) ([]platform.WindowRecord, error) {
	listed, err := m.AllWindows(ctx)
	if err != nil {
		return nil, err
	}
	var matching []platform.WindowRecord
	for _, record := range listed {
		if record.PID == pid {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

func (m macWindows) AllWindows(ctx context.Context) ([]platform.WindowRecord, error) {
	listed, err := call[[]swifthelper.ListedWindow](ctx, m.macHelper, "all")
	if err != nil {
		return nil, err
	}
	records := make([]platform.WindowRecord, 0, len(listed))
	for _, window := range listed {
		records = append(records, window.PlatformWindow(window.PID, window.AppName, window.ZIndex))
	}
	return records, nil
}

func (m macWindows) FocusedWindow(ctx context.Context, pid int) (platform.WindowRecord, error) {
	window, err := call[swifthelper.Window](ctx, m.macHelper, swifthelper.ListArgs("focused", pid)...)
	if err != nil {
		return platform.WindowRecord{}, err
	}
	return window.PlatformWindow(pid, "", 0), nil
}

// Frontmost names the app in front from Launch Services, as the Electron app does. The window listing has no
// usable front app: its first window is often a system window that no listed app owns.
func (m macWindows) Frontmost(ctx context.Context) (platform.WindowApp, bool, error) {
	return frontmostApp(ctx, m.helper)
}

func (m macWindows) SetFrame(ctx context.Context, write platform.WindowWrite) (platform.WindowFrame, error) {
	return m.frameCall(ctx, swifthelper.SetFrameArgs(write)...)
}

func (m macWindows) SetFullScreen(ctx context.Context, pid int, key string, fullScreen bool) (platform.WindowFrame, error) {
	return m.frameCall(ctx, swifthelper.SetFullScreenArgs(pid, key, fullScreen)...)
}

// frameCall returns the frame the helper reports. An "unapplied" outcome is a failure, as the helper means it.
func (m macWindows) frameCall(ctx context.Context, args ...string) (platform.WindowFrame, error) {
	result, err := call[swifthelper.FrameResult](ctx, m.macHelper, args...)
	if err != nil {
		return platform.WindowFrame{}, err
	}
	if result.Outcome == "unapplied" {
		return platform.WindowFrame{}, &platform.WindowError{Code: platform.WindowErrorUnapplied, Message: result.Reason}
	}
	return result.Frame.Platform(), nil
}

func (m macWindows) Raise(ctx context.Context, pid int, key string) error {
	envelope, err := m.run(ctx, swifthelper.RaiseArgs(pid, key)...)
	if err != nil {
		return err
	}
	return envelope.Err()
}
