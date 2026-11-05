package real

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// shell uses the default applications and file manager through MyGo.
type shell struct{}

// OpenPath opens a file or folder with its default application.
func (shell) OpenPath(path string) error { return mygo.Shell.OpenPath(path) }

// OpenURL opens a web address in the default browser.
func (shell) OpenURL(url string) error { return mygo.Shell.OpenExternal(url) }

// Reveal selects an item in the file manager. MyGo reports no error for it.
func (shell) Reveal(path string) error {
	mygo.Shell.ShowItemInFolder(path)
	return nil
}

// QuickLook shows an item in the system quick preview. Only macOS has one; see quicklook_darwin.go.
func (shell) QuickLook(path string) error { return quickLook(path) }

// OpenWith opens a file with one application where the OS can choose it; see openwith_darwin.go.
func (shell) OpenWith(path string, app platform.App) error { return openWith(path, app) }

// Trash moves an item to the Trash. MyGo refuses to delete permanently when the OS has no Trash for it.
func (shell) Trash(path string) error { return mygo.Shell.TrashItem(path) }

// InstalledApps lists the launchable applications, sorted by name.
func (shell) InstalledApps() ([]platform.App, error) {
	found, err := mygo.Shell.InstalledApps()
	if err != nil {
		return nil, err
	}
	apps := make([]platform.App, 0, len(found))
	for _, app := range found {
		apps = append(apps, platform.App{Name: app.Name, ID: app.ID, Path: app.Path})
	}
	return apps, nil
}

// AppIcon returns the file manager's icon as PNG.
func (shell) AppIcon(path string, size int) ([]byte, error) {
	return mygo.Shell.FileIcon(path, size)
}

// LaunchApp starts an application or brings it forward.
func (shell) LaunchApp(app platform.App) error {
	return mygo.Shell.LaunchApp(mygo.InstalledApp{Name: app.Name, ID: app.ID, Path: app.Path})
}

// Beep plays the system alert sound.
func (shell) Beep() { mygo.Shell.Beep() }
