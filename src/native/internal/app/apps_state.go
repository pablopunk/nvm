package app

import (
	"context"
	"os"
	"runtime"
	"strings"

	"github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// The apps feature's lifecycle in the app: the services are built over the ports and the app's store, the saved list
// loads off the UI thread, and the running state is polled only while the palette is visible.

// startAppServices builds the apps services and starts loading the saved list. It runs before the palette exists, so
// the palette's show hook can use the services.
func (p *App) startAppServices() {
	p.ports = real.NewPorts()
	var cache apps.Cache
	if p.appStore != nil {
		cache = apps.NewCollectionCache(apps.NewCacheDocument(p.appStore))
	}
	home, _ := os.UserHomeDir()
	p.appsvc = apps.NewServices(apps.ServicesOptions{
		Ports:            p.ports,
		Cache:            cache,
		Home:             home,
		Platform:         runtime.GOOS,
		UID:              os.Getuid(),
		NevermindAppPath: nevermindAppBundle(),
		Hooks: apps.IndexHooks{
			InvalidateRunning:      func() { p.appsvc.Running.Invalidate() },
			ScheduleRunningRefresh: func(reason string) { p.appsvc.Running.ScheduleRefresh(reason) },
			Indexed:                func(int) { p.onAppsChanged() },
			Error: func(err error) {
				logx.Default().Warn("apps.index.failed", logx.F{"error": err.Error()})
			},
		},
		Changed: p.onAppsChanged,
	})
	safego.Go("apps.cache", func(context.Context) {
		if err := p.appsvc.Index.LoadCache(); err != nil {
			logx.Default().Warn("apps.cache.unreadable", logx.F{"error": err.Error()})
		}
		p.onAppsChanged()
	})
}

// onAppsChanged asks the UI thread to rebuild the rows that show apps. Safe from any goroutine; a no-op before the
// window exists.
func (p *App) onAppsChanged() {
	if p.panel == nil {
		return
	}
	p.panel.Update(func() { p.ReloadRows() })
}

// showApps runs when the palette is shown: the index refreshes (debounced), the running state refreshes, and the
// running state is polled every RunningPollInterval until the palette hides.
func (p *App) showApps() {
	if p.appsvc == nil {
		return
	}
	p.appsvc.Index.RequestRefresh()
	p.appsvc.Running.ScheduleRefresh("palette-shown")
	if p.stopAppPoll == nil {
		p.stopAppPoll = apps.Poll(p.ports.Clock, apps.RunningPollInterval, func() {
			p.appsvc.Running.ScheduleRefresh("visible-poll")
		})
	}
}

// hideApps runs when the palette hides: the running poll stops, so nothing polls while the palette is hidden.
func (p *App) hideApps() {
	if stop := p.stopAppPoll; stop != nil {
		p.stopAppPoll = nil
		stop()
	}
}

// appsSnapshot returns the apps the root's providers search: a copy of the index, taken on the UI thread.
func (p *App) appsSnapshot() []apps.App {
	if p.appsvc == nil {
		return nil
	}
	return p.appsvc.Index.Apps()
}

// nevermindAppBundle returns the path of the .app this process runs from, or "" when it is not in a bundle.
func nevermindAppBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if i := strings.Index(exe, ".app/"); i >= 0 {
		return exe[:i+len(".app")]
	}
	if strings.HasSuffix(exe, ".app") {
		return exe
	}
	return ""
}
