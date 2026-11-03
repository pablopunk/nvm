package app

import (
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
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
