package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// AppLifecycle reads and controls the installed apps the host indexed: running state, quit, force quit, quit all and
// hide. The apps feature implements it; a build without one answers unavailable.
type AppLifecycle interface {
	IndexedApps() []appfeature.App
	DiscoverUninstall(appPath string) appfeature.Discovery
	TrashUninstall(snapshot appfeature.UninstallSnapshot, values map[string]bool) appfeature.TrashResult
	TrashFolder() string
	RunningApps(list []appfeature.App) ([]appfeature.App, error)
	QuitApp(app appfeature.App) (appfeature.QuitOutcome, error)
	ForceQuitApp(app appfeature.App) (appfeature.QuitOutcome, error)
	QuitAllApps() ([]appfeature.App, error)
	HideApp(app appfeature.App) error
}

// runningPaths returns the paths of the indexed apps that run, in request order. Paths the index does not list are
// not running.
func (h *Handler) runningPaths(ctx context.Context, raw json.RawMessage) ([]string, error) {
	var args protocol.PathsArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if len(args.Paths) > api.MaxRunningPaths {
		return nil, invalid("at most %d app paths can be checked at once", api.MaxRunningPaths)
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppRunning, "running status")
	if err != nil {
		return nil, err
	}
	listed := h.listedApps(args.Paths)
	if err := h.admitDesktopEffect(ctx); err != nil {
		return nil, err
	}
	running, err := lifecycle.RunningApps(listed)
	if err != nil {
		return nil, h.lifecycleFailure("running status", err)
	}
	return runningPathsInRequestOrder(args.Paths, running), nil
}

// quitApp quits or force quits one indexed app, under the desktop action rate limit.
func (h *Handler) quitApp(ctx context.Context, raw json.RawMessage, force bool) (api.QuitResult, error) {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.QuitResult{}, err
	}
	action := "quit"
	if force {
		action = "force quit"
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppQuit, action)
	if err != nil {
		return api.QuitResult{}, err
	}
	app, err := h.indexedApp(args.Path)
	if err != nil {
		return api.QuitResult{}, err
	}
	if !h.actions.allow() {
		return api.QuitResult{}, rateExceeded("app actions")
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return api.QuitResult{}, err
	}
	outcome, err := quitFunc(lifecycle, force)(featureApp(app))
	if err != nil {
		return api.QuitResult{}, h.lifecycleFailure(action, err)
	}
	return quitResultOf(outcome), nil
}

// quitAllApps asks every running app to quit. It needs the system declaration, as the 1.x Quit All command did.
func (h *Handler) quitAllApps(ctx context.Context) ([]api.App, error) {
	if err := h.require(api.CapabilitySystem); err != nil {
		return nil, err
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppQuit, "quit all")
	if err != nil {
		return nil, err
	}
	if !h.actions.allow() {
		return nil, rateExceeded("app actions")
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return nil, err
	}
	asked, err := lifecycle.QuitAllApps()
	if err != nil {
		return nil, h.lifecycleFailure("quit all", err)
	}
	return apiApps(asked), nil
}

// hideApp hides one indexed app's windows. Only macOS can.
func (h *Handler) hideApp(ctx context.Context, raw json.RawMessage) error {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppHide, "hide")
	if err != nil {
		return err
	}
	app, err := h.indexedApp(args.Path)
	if err != nil {
		return err
	}
	if !h.actions.allow() {
		return rateExceeded("app actions")
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return err
	}
	if err := lifecycle.HideApp(featureApp(app)); err != nil {
		return h.lifecycleFailure("hide", err)
	}
	return nil
}

// lifecycle returns the app lifecycle port once the system supports the intent. A missing port or an unsupported
// intent answers unavailable, with the words the palette shows.
func (h *Handler) lifecycle(capability platform.Capability, action string) (AppLifecycle, error) {
	if h.deps.Apps == nil {
		return nil, unavailable("apps cannot be %s in this build", action)
	}
	if !h.appLifecycleSupported(capability) {
		return nil, unavailable("%s is not available on this system yet", action)
	}
	return h.deps.Apps, nil
}

// listedApps returns the indexed apps among paths. A path the index does not list is left out.
func (h *Handler) listedApps(paths []string) []appfeature.App {
	var out []appfeature.App
	for _, path := range paths {
		if app, err := h.indexedApp(path); err == nil {
			out = append(out, featureApp(app))
		}
	}
	return out
}

// lifecycleFailure reports an app action the OS refused. A script's own reason goes to the HUD and the log, as the
// AppleScript failures of the host do. Any other failure is logged and shown with its message.
func (h *Handler) lifecycleFailure(action string, err error) error {
	var script appfeature.ScriptFailure
	if errors.As(err, &script) {
		result := api.ShellResult{ExitCode: script.ExitCode, Stderr: script.Stderr}
		h.reportAppleScriptFailure(result)
		return unavailable("%s", appleScriptReason(result))
	}
	reason := err.Error()
	h.logger.Warn("the app action failed", logx.F{"action": action, "reason": reason})
	if h.deps.Notify != nil {
		h.deps.Notify(h.manifest.Title, reason)
	}
	if errors.Is(err, platform.ErrDenied) {
		return permissionDenied(reason)
	}
	return unavailable("%s", reason)
}

// quitFunc picks the quit or force quit of the port.
func quitFunc(lifecycle AppLifecycle, force bool) func(appfeature.App) (appfeature.QuitOutcome, error) {
	if force {
		return lifecycle.ForceQuitApp
	}
	return lifecycle.QuitApp
}

// quitResultOf says a quit outcome in the API's terms. The app is quit when every process it had is gone, by exit or
// by a kill.
func quitResultOf(outcome appfeature.QuitOutcome) api.QuitResult {
	return api.QuitResult{
		WasRunning: outcome.Found > 0,
		Quit:       outcome.Found > 0 && outcome.Failed == 0 && outcome.Terminated+outcome.Killed == outcome.Found,
		Killed:     outcome.Killed > 0,
	}
}

// runningPathsInRequestOrder keeps the requested paths that the running apps include.
func runningPathsInRequestOrder(requested []string, running []appfeature.App) []string {
	marked := map[string]bool{}
	for _, app := range running {
		marked[app.Path] = true
	}
	out := []string{}
	for _, path := range requested {
		if marked[path] {
			out = append(out, path)
		}
	}
	return out
}

func featureApp(app platform.App) appfeature.App {
	return appfeature.App{ID: app.ID, Name: app.Name, Path: app.Path}
}

func apiApps(list []appfeature.App) []api.App {
	out := make([]api.App, 0, len(list))
	for _, app := range list {
		out = append(out, api.App{ID: app.ID, Name: app.Name, Path: app.Path})
	}
	return out
}

func (h *Handler) appLifecycleSupported(capability platform.Capability) bool {
	return h.deps.Apps != nil && h.deps.Capabilities.Supported(capability)
}
