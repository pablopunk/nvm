package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// appIconSize is the edge in pixels of the icons extensions receive; maxListedApps bounds one listing.
const (
	appIconSize   = 64
	maxListedApps = 500
)

func (h *Handler) serveDesktopApps(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopApps); err != nil {
		return nil, err
	}
	if h.deps.Shell == nil && h.deps.Apps == nil {
		return nil, unavailable("apps are not available in this build")
	}
	switch call.Op {
	case protocol.OpList:
		return h.installedApps()
	case protocol.OpSearch:
		return h.searchApps(call.Args)
	case protocol.OpLaunch:
		return nil, h.launchApp(ctx, call.Args)
	case protocol.OpIcon:
		return h.appIcon(call.Args)
	case protocol.OpFrontmost:
		return h.frontmostApp(ctx)
	case protocol.OpRunning:
		return h.runningPaths(ctx, call.Args)
	case protocol.OpQuit:
		return h.quitApp(ctx, call.Args, false)
	case protocol.OpForceQuit:
		return h.quitApp(ctx, call.Args, true)
	case protocol.OpQuitAll:
		return h.quitAllApps(ctx)
	case protocol.OpHideApp:
		return nil, h.hideApp(ctx, call.Args)
	case protocol.OpUninstallReview, protocol.OpUninstallTrash:
		return h.serveUninstall(call)
	}
	return nil, invalid("desktop.apps has no operation %q", call.Op)
}

// installedApps lists the apps the host index lists, with the time each was first seen. Without an apps lifecycle the
// platform's own listing answers.
func (h *Handler) installedApps() ([]api.App, error) {
	if h.deps.Apps != nil {
		return indexedAppList(h.deps.Apps.IndexedApps()), nil
	}
	apps, err := h.deps.Shell.InstalledApps()
	if err != nil {
		return nil, fmt.Errorf("list the installed apps: %w", err)
	}
	out := make([]api.App, 0, min(len(apps), maxListedApps))
	for _, app := range apps {
		if len(out) == maxListedApps {
			break
		}
		out = append(out, api.App{ID: app.ID, Name: app.Name, Path: app.Path})
	}
	return out, nil
}

func indexedAppList(list []appfeature.App) []api.App {
	out := make([]api.App, 0, min(len(list), maxListedApps))
	for _, app := range list {
		if len(out) == maxListedApps {
			break
		}
		out = append(out, api.App{ID: app.ID, Name: app.Name, Path: app.Path, FirstSeenMs: app.FirstSeenAt})
	}
	return out
}

func (h *Handler) searchApps(raw json.RawMessage) ([]api.App, error) {
	var args protocol.QueryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	apps, err := h.installedApps()
	if err != nil {
		return nil, err
	}
	return appsNamed(apps, args.Query), nil
}

// appsNamed keeps the apps whose name contains query, ignoring case.
func appsNamed(apps []api.App, query string) []api.App {
	needle := strings.ToLower(strings.TrimSpace(query))
	out := []api.App{}
	for _, app := range apps {
		if strings.Contains(strings.ToLower(app.Name), needle) {
			out = append(out, app)
		}
	}
	return out
}

func (h *Handler) launchApp(ctx context.Context, raw json.RawMessage) error {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	app, err := h.indexedApp(args.Path)
	if err != nil {
		return err
	}
	if !h.actions.allow() {
		return rateExceeded("app launches")
	}
	if h.deps.Shell == nil {
		return unavailable("apps cannot be launched in this build")
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return err
	}
	err = h.deps.Shell.LaunchApp(app)
	if errors.Is(err, platform.ErrDenied) {
		return permissionDenied(err.Error())
	}
	if errors.Is(err, platform.ErrUnsupported) {
		return unavailable("%s", err.Error())
	}
	return err
}

func (h *Handler) appIcon(raw json.RawMessage) (protocol.IconResult, error) {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.IconResult{}, err
	}
	app, err := h.indexedApp(args.Path)
	if err != nil {
		return protocol.IconResult{}, err
	}
	if h.deps.Shell == nil {
		return protocol.IconResult{}, nil
	}
	png, err := h.deps.Shell.AppIcon(app.Path, appIconSize)
	if err != nil || len(png) == 0 {
		return protocol.IconResult{}, nil
	}
	return protocol.IconResult{DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), Found: true}, nil
}

// indexedApp returns the installed app at path. Only apps the index lists can be launched or read, so an extension
// cannot start an arbitrary binary by naming its path.
func (h *Handler) indexedApp(path string) (platform.App, error) {
	if !filepath.IsAbs(path) {
		return platform.App{}, invalid("an app path must be absolute")
	}
	apps, err := h.indexedPlatformApps()
	if err != nil {
		return platform.App{}, err
	}
	for _, app := range apps {
		if filepath.Clean(app.Path) == filepath.Clean(path) {
			return app, nil
		}
	}
	return platform.App{}, notFound("no installed app is at %s", path)
}

// indexedAppList is the list of apps the host index lists, as the platform's app records. Without an apps lifecycle the
// platform's own listing answers.
func (h *Handler) indexedPlatformApps() ([]platform.App, error) {
	if h.deps.Apps != nil {
		list := h.deps.Apps.IndexedApps()
		out := make([]platform.App, 0, len(list))
		for _, app := range list {
			out = append(out, platform.App{ID: app.ID, Name: app.Name, Path: app.Path})
		}
		return out, nil
	}
	apps, err := h.deps.Shell.InstalledApps()
	if err != nil {
		return nil, fmt.Errorf("list the installed apps: %w", err)
	}
	return apps, nil
}
