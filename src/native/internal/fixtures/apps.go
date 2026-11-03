package fixtures

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// DefaultAppCount is how many apps SeededSet installs.
const DefaultAppCount = 40

// appIconSize is the edge length of generated app icons, in pixels.
const appIconSize = 32

// appNames are the first names given to fixture apps; later apps are numbered.
var appNames = []string{
	"Calendar", "Notes", "Mail", "Maps", "Music", "Photos", "Terminal", "Calculator", "Clock", "Contacts",
	"Reminders", "Safari", "Messages", "FaceTime", "Preview", "TextEdit", "Finder", "Settings", "Weather", "Stocks",
	"Voice Memos", "Books", "Podcasts", "Freeform", "Home", "Shortcuts", "Activity Monitor", "Disk Utility",
}

// AppFixture is a generated application: its listing and the PNG icon the shell returns for it.
type AppFixture struct {
	App  platform.App
	Icon []byte
}

// Apps returns n applications, sorted by name as the shell lists them. Names come from a fixed list and
// are numbered past its end. Each app has an icon generated from its name.
func Apps(n int) []AppFixture {
	apps := make([]AppFixture, 0, n)
	for i := range n {
		name := fixtureAppName(i)
		slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		path := "/Applications/" + name + ".app"
		apps = append(apps, AppFixture{
			App:  platform.App{Name: name, ID: "com.example.fixture." + slug, Path: path},
			Icon: iconPNG(name, appIconSize),
		})
	}
	slices.SortStableFunc(apps, func(a, b AppFixture) int { return strings.Compare(a.App.Name, b.App.Name) })
	return apps
}

// SeedShell loads apps into a fake shell: its listing, and icons keyed by app path.
func SeedShell(shell *fake.Shell, apps []AppFixture) {
	listing := make([]platform.App, 0, len(apps))
	icons := make(map[string][]byte, len(apps))
	for _, app := range apps {
		listing = append(listing, app.App)
		icons[app.App.Path] = app.Icon
	}
	shell.SetApps(listing, icons)
}

// SeededSet returns a fake platform set with DefaultAppCount apps installed in its shell. Other fakes start
// empty; load them with the builders in this package.
func SeededSet() *fake.Set {
	set := fake.NewSet()
	SeedShell(set.Shell, Apps(DefaultAppCount))
	return set
}

// fixtureAppName returns the name of the i-th app.
func fixtureAppName(i int) string {
	if i < len(appNames) {
		return appNames[i]
	}
	return fmt.Sprintf("Fixture App %d", i+1)
}
