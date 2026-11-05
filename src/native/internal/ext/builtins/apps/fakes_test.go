package apps

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeApps serves the apps the test lists and records the calls the built-in makes.
type fakeApps struct {
	api.Apps
	list       []api.App
	running    map[string]bool
	launched   []string
	forced     []string
	quitAsked  int
	review     api.UninstallReview
	reviewErr  error
	trashed    []string
	trashToken string
	result     api.UninstallResult
	forceQuit  api.QuitResult
}

func (f *fakeApps) List() ([]api.App, error) { return f.list, nil }

func (f *fakeApps) Running(paths []string) ([]string, error) {
	var out []string
	for _, path := range paths {
		if f.running[path] {
			out = append(out, path)
		}
	}
	return out, nil
}

func (f *fakeApps) Launch(path string) error {
	f.launched = append(f.launched, path)
	return nil
}

func (f *fakeApps) ForceQuit(path string) (api.QuitResult, error) {
	f.forced = append(f.forced, path)
	return f.forceQuit, nil
}

func (f *fakeApps) QuitAll() ([]api.App, error) {
	f.quitAsked++
	return f.list, nil
}

func (f *fakeApps) UninstallReview(string) (api.UninstallReview, error) {
	return f.review, f.reviewErr
}

func (f *fakeApps) UninstallTrash(token string, ids []string) (api.UninstallResult, error) {
	f.trashToken = token
	f.trashed = ids
	return f.result, nil
}

// fakeFiles shortens home-folder paths, as the host does.
type fakeFiles struct{ api.Files }

func (fakeFiles) DisplayPath(path string) string { return path }

type fakeDesktop struct {
	api.Desktop
	apps  *fakeApps
	files fakeFiles
}

func (d fakeDesktop) Apps() api.Apps   { return d.apps }
func (d fakeDesktop) Files() api.Files { return d.files }

type fakeSystem struct {
	api.System
	capabilities map[string]bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.capabilities[name] }

type fakeShortcuts struct {
	api.Shortcuts
	aliases map[string][]string
}

func (s fakeShortcuts) Aliases(prefix string) map[string][]string {
	out := map[string][]string{}
	for id, phrases := range s.aliases {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			out[id] = phrases
		}
	}
	return out
}

type fakeNavigation struct{ api.Navigation }

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

// fakeViewsContext is the context the built-in runs with in these tests.
type fakeViewsContext struct {
	api.Context
	apps      *fakeApps
	system    fakeSystem
	shortcuts fakeShortcuts
}

func (c fakeViewsContext) Desktop() api.Desktop {
	return fakeDesktop{apps: c.apps}
}
func (c fakeViewsContext) System() api.System         { return c.system }
func (c fakeViewsContext) Shortcuts() api.Shortcuts   { return c.shortcuts }
func (c fakeViewsContext) Navigation() api.Navigation { return fakeNavigation{} }

// newTestContext builds a context over the given apps, with uninstall supported.
func newTestContext(apps *fakeApps) fakeViewsContext {
	return fakeViewsContext{
		apps:      apps,
		system:    fakeSystem{capabilities: map[string]bool{"apps.uninstall": true}},
		shortcuts: fakeShortcuts{aliases: map[string][]string{}},
	}
}
