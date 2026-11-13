package windowmgmt

import (
	"context"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// MaxListedApps bounds how many apps one listing reads, so a machine with many apps cannot fan out window
// system calls.
const MaxListedApps = 40

// WindowQuery filters ListWindows. An empty field does not filter. AppID matches the bundle identifier
// exactly, AppName and Title match substrings, all case-insensitively.
type WindowQuery struct {
	AppID   string
	AppName string
	Title   string
}

// WindowDescriptor is a listed window. Handle names it for ApplyToWindow and FocusWindow until it expires.
type WindowDescriptor struct {
	Handle     string
	AppID      string
	AppName    string
	Title      string
	Frame      geometry.Rect
	DisplayID  string
	FullScreen bool
	Minimized  bool
}

// ListDisplays returns the connected displays with their work areas.
func (s *Service) ListDisplays(ctx context.Context) ([]geometry.Display, error) {
	displays, err := s.listDisplays(ctx)
	s.reportFailure("list-displays", err)
	return displays, err
}

func (s *Service) listDisplays(ctx context.Context) ([]geometry.Display, error) {
	if !s.Available() {
		return nil, newFailure(ReasonUnsupported, "")
	}
	if failed := s.accessFailure(ctx); failed != nil {
		return nil, failed
	}
	displays, err := s.windows.Displays(ctx)
	if err != nil {
		return nil, failureOf(err)
	}
	return displaysFromPlatform(displays), nil
}

// ListWindows returns the windows of the running apps that match the query, each with a fresh handle.
func (s *Service) ListWindows(ctx context.Context, query WindowQuery) ([]WindowDescriptor, error) {
	windows, err := s.listWindows(ctx, query)
	s.reportFailure("list-windows", err)
	return windows, err
}

func (s *Service) listWindows(ctx context.Context, query WindowQuery) ([]WindowDescriptor, error) {
	if !s.Available() {
		return nil, newFailure(ReasonUnsupported, "")
	}
	if failed := s.accessFailure(ctx); failed != nil {
		return nil, failed
	}
	apps, err := s.windows.RunningApps(ctx)
	if err != nil {
		return nil, failureOf(err)
	}
	descriptors := []WindowDescriptor{}
	for _, app := range matchingApps(apps, query) {
		descriptors = append(descriptors, s.describeAppWindows(ctx, app, query.Title)...)
	}
	return descriptors, nil
}

// describeAppWindows lists one app's matching windows. An app that cannot be read is skipped, so one
// misbehaving app does not hide the others.
func (s *Service) describeAppWindows(ctx context.Context, app platform.WindowApp, title string) []WindowDescriptor {
	records, err := s.windows.Windows(ctx, app.PID)
	if err != nil {
		return nil
	}
	descriptors := []WindowDescriptor{}
	for _, record := range records {
		if !containsFold(record.Title, title) {
			continue
		}
		descriptors = append(descriptors, s.describeWindow(app, record))
	}
	return descriptors
}

func (s *Service) describeWindow(app platform.WindowApp, record platform.WindowRecord) WindowDescriptor {
	handle := s.handles.issue(handleTarget{PID: app.PID, BundleID: app.BundleID, Key: record.Key})
	return WindowDescriptor{
		Handle:     handle,
		AppID:      app.BundleID,
		AppName:    app.Name,
		Title:      record.Title,
		Frame:      rectFromPlatform(record.Frame),
		DisplayID:  record.DisplayID,
		FullScreen: record.FullScreen,
		Minimized:  record.Minimized,
	}
}

func matchingApps(apps []platform.WindowApp, query WindowQuery) []platform.WindowApp {
	matching := []platform.WindowApp{}
	for _, app := range apps {
		if matchesAppID(app, query.AppID) && containsFold(app.Name, query.AppName) {
			matching = append(matching, app)
		}
	}
	if len(matching) > MaxListedApps {
		matching = matching[:MaxListedApps]
	}
	return matching
}

func matchesAppID(app platform.WindowApp, appID string) bool {
	return appID == "" || strings.EqualFold(app.BundleID, appID)
}

// containsFold reports whether needle is empty or occurs in haystack, ignoring case.
func containsFold(haystack, needle string) bool {
	return needle == "" || strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
