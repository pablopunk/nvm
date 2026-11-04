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

// TestForceQuitEnterAsksForTheForceQuitConfirmation: Enter on a running app in Force Quit Apps asks to force quit it,
// as 1.x does; it does not quit the app gently.
func TestForceQuitEnterAsksForTheForceQuitConfirmation(t *testing.T) {
	p := runningGoldenPalette(t, goldenFirefoxPath)
	goldenOpen(p, p.forceQuitView())
	rows := p.Results()
	if len(rows) != 1 || rows[0].Title != "Firefox" || rows[0].Subtitle != "Force quit this application" {
		t.Fatalf("rows = %v", rowTitles(p))
	}
	if len(rows[0].Actions) != 1 {
		t.Fatalf("actions = %d, want only Force Quit", len(rows[0].Actions))
	}
	rows[0].Actions[0].Run(p)
	if top := p.Top(); top.Title != "Confirm" || top.Placeholder != "Force Quit?" {
		t.Fatalf("top view = %q / %q, want the Force Quit confirmation", top.Title, top.Placeholder)
	}
	if confirm := p.Results()[0]; confirm.Title != "Force Quit" || confirm.Subtitle != "Force quit Firefox? Unsaved data may be lost." {
		t.Fatalf("confirmation row = %q / %q", confirm.Title, confirm.Subtitle)
	}
}
