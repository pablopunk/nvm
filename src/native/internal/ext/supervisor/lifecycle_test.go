package supervisor

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fixedApps is the app lifecycle port the fixture sees: Notes runs, quits and hides, and Quit All asks it.
type fixedApps struct{}

func (fixedApps) RunningApps(list []appfeature.App) ([]appfeature.App, error) { return list, nil }

func (fixedApps) QuitApp(appfeature.App) (appfeature.QuitOutcome, error) {
	return appfeature.QuitOutcome{Found: 1, Terminated: 1}, nil
}

func (fixedApps) ForceQuitApp(appfeature.App) (appfeature.QuitOutcome, error) {
	return appfeature.QuitOutcome{Found: 1, Killed: 1}, nil
}

func (fixedApps) QuitAllApps() ([]appfeature.App, error) {
	return []appfeature.App{{Name: "Notes", Path: "/Applications/Notes.app"}}, nil
}

func (fixedApps) HideApp(appfeature.App) error { return nil }

func (fixedApps) IndexedApps() []appfeature.App { return nil }

func (fixedApps) DiscoverUninstall(string) appfeature.Discovery { return appfeature.Discovery{} }

func (fixedApps) TrashUninstall(appfeature.UninstallSnapshot, map[string]bool) appfeature.TrashResult {
	return appfeature.TrashResult{}
}

// TestLifecycleFixtureUsesTheAppServicesThroughTheChild runs the lifecycle fixture in its child. Each probe reads what
// the host answered; without the declarations every probe is refused before the port is reached.
func TestLifecycleFixtureUsesTheAppServicesThroughTheChild(t *testing.T) {
	rig := newServicesRig(t)
	shell := fake.NewShell()
	shell.SetApps([]platform.App{{Name: "Notes", ID: "com.example.notes", Path: "/Applications/Notes.app"}}, nil)
	rig.deps.Shell = shell
	rig.deps.Apps = fixedApps{}
	rig.deps.Capabilities = platform.NewCapabilities(platform.CapabilityAppRunning, platform.CapabilityAppQuit, platform.CapabilityAppHide)

	declared := servicedRuntime(t, "acme.lifecycle", rig, fixtureManifest("acme.lifecycle", api.CapabilityDesktopApps, api.CapabilitySystem))
	want := map[string]string{
		"running":  "running=1",
		"quit":     "quit was=true quit=true",
		"hide":     "hide sent=true",
		"quit-all": "quit-all asked=1",
	}
	for query, title := range want {
		if got := rowTitle(t, declared, query); got != title {
			t.Errorf("%s row = %q, want %q", query, got, title)
		}
	}

	undeclared := servicedRuntime(t, "acme.lifecycle", rig, fixtureManifest("acme.lifecycle"))
	for query := range want {
		if got := rowTitle(t, undeclared, query); !strings.Contains(got, "failed") {
			t.Errorf("undeclared %s row = %q, want a refusal", query, got)
		}
	}
}
