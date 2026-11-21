package app

import (
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

// newFixtureAppServices builds the apps services over fake ports, with list as the saved index. The goldens and the
// benchmarks use it: nothing is scanned, nothing is polled, and nothing touches the machine.
func newFixtureAppServices(list []appfeat.App) *appfeat.Services {
	cache := &appfeat.MemoryCache{}
	_ = cache.Save(appfeat.Snapshot{
		Version:   appfeat.SnapshotVersion,
		Apps:      list,
		FirstSeen: appfeat.FirstSeen{ByID: map[string]int64{}, Initialized: true},
	})
	services := appfeat.NewServices(appfeat.ServicesOptions{
		Ports:    fake.NewSet().Ports(),
		Cache:    cache,
		Home:     "/home/tester",
		Platform: "linux",
	})
	_ = services.Index.LoadCache()
	return services
}

func TestWarmAppCatalogLoadsCachedAppsAndStartsScan(t *testing.T) {
	loom := appfeat.App{ID: "dev.loom", Name: "Loom", Path: "/Applications/Loom.app"}
	cache := &appfeat.MemoryCache{}
	_ = cache.Save(appfeat.Snapshot{Version: appfeat.SnapshotVersion, Apps: []appfeat.App{loom}, FirstSeen: appfeat.FirstSeen{ByID: map[string]int64{}, Initialized: true}})
	started, release := make(chan struct{}), make(chan struct{})
	index := appfeat.NewIndex(appfeat.IndexOptions{Clock: fake.NewClock(), Cache: cache, Scan: func() (appfeat.ScanResult, error) {
		close(started)
		<-release
		return appfeat.ScanResult{Apps: []appfeat.App{loom}, Complete: true}, nil
	}})
	p := &App{appsvc: &appfeat.Services{Index: index}}
	p.warmAppCatalog()
	<-started
	if got := index.Apps(); len(got) != 1 || got[0].ID != loom.ID {
		t.Fatalf("cached apps = %+v", got)
	}
	close(release)
	index.WaitIdle()
}
