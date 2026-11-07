package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// frontmostTimeout bounds the lookup. On macOS it runs two Launch Services queries.
const frontmostTimeout = 5 * time.Second

// frontmostApp answers the app in front. It shares the desktop rate limit and needs the host's frontmost port and the
// frontmost-app capability; the caller has already checked the apps declaration.
func (h *Handler) frontmostApp(ctx context.Context) (api.App, error) {
	if h.deps.Frontmost == nil || !h.deps.Capabilities.Supported(platform.CapabilityFrontmostApp) {
		return api.App{}, unavailable("the frontmost app is not available on this system")
	}
	if !h.actions.allow() {
		return api.App{}, rateExceeded("frontmost app reads")
	}
	ctx, cancel := context.WithTimeout(ctx, frontmostTimeout)
	defer cancel()
	window, found, err := h.deps.Frontmost.Frontmost(ctx)
	if err != nil {
		return api.App{}, fmt.Errorf("read the frontmost app: %w", err)
	}
	if !found {
		return api.App{}, notFound("no app is in front")
	}
	return h.describeFrontmost(window), nil
}

// describeFrontmost gives the indexed app's id, name and path when the index lists the bundle id, so the icon call
// works. An app the index does not list keeps its bundle id as its name, with an empty path.
func (h *Handler) describeFrontmost(window platform.WindowApp) api.App {
	if indexed, ok := h.indexedAppByID(window.BundleID); ok {
		return api.App{ID: indexed.ID, Name: indexed.Name, Path: indexed.Path}
	}
	name := window.Name
	if name == "" {
		name = window.BundleID
	}
	return api.App{ID: window.BundleID, Name: name}
}

// indexedAppByID finds the installed app with this id, ignoring case because bundle ids are compared that way.
func (h *Handler) indexedAppByID(id string) (platform.App, bool) {
	if id == "" || h.deps.Shell == nil {
		return platform.App{}, false
	}
	apps, err := h.deps.Shell.InstalledApps()
	if err != nil {
		return platform.App{}, false
	}
	for _, app := range apps {
		if strings.EqualFold(app.ID, id) {
			return app, true
		}
	}
	return platform.App{}, false
}
