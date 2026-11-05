package app

import (
	"testing"

	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// runningGoldenPalette is the golden palette with the given golden apps (by path) reported as running.
func runningGoldenPalette(t *testing.T, running ...string) *App {
	t.Helper()
	p := goldenPalette(t)
	live := map[string]bool{}
	for _, path := range running {
		live[path] = true
	}
	p.appsvc.Running = appfeat.NewRunning(appfeat.RunningOptions{
		Candidates: func() []appfeat.App { return goldenApps() },
		Clock:      fake.NewSet().Clock,
		Platform:   "linux",
		Detect: func([]appfeat.App) (map[string]bool, error) {
			return live, nil
		},
	})
	p.appsvc.Running.Refresh("test")
	return p
}

const goldenFirefoxPath = "/usr/share/applications/firefox.desktop"
