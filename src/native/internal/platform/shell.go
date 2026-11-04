package platform

// App is an application installed on the computer, as a launcher lists it.
type App struct {
	// Name is the name the system shows, in the user's language.
	Name string
	// ID is the stable identifier the OS uses for the app (bundle ID, AppUserModelID, desktop file ID).
	ID string
	// Path is where the app is: its bundle, its shell item, or its desktop entry.
	Path string
}

// Shell is the intent "use the computer's default applications and file manager": open files and
// links, reveal items, move things to the Trash, list and launch apps, show app icons, and beep.
// Installed apps and icons are capabilities shell.installedApps and shell.appIcons; Trash is
// shell.trash; Reveal is shell.reveal.
type Shell interface {
	// OpenPath opens a file or folder with its default application.
	OpenPath(path string) error
	// OpenURL opens a web address in the default browser.
	OpenURL(url string) error
	// Reveal selects an item in the system file manager.
	Reveal(path string) error
	// QuickLook shows an item in the system's quick preview, without opening it. Only systems with the
	// shell.quickLook capability have one; the others return ErrUnsupported.
	QuickLook(path string) error
	// Trash moves an item to the Trash or Recycle Bin. It must not delete permanently; an item that
	// cannot be trashed is left in place and an error is returned.
	Trash(path string) error
	// InstalledApps lists the applications the user can launch, sorted by name.
	InstalledApps() ([]App, error)
	// AppIcon returns the icon of a file, folder or application as PNG at size by size pixels.
	AppIcon(path string, size int) ([]byte, error)
	// LaunchApp starts an installed application, or brings it forward when it runs.
	LaunchApp(app App) error
	// Beep plays the system alert sound.
	Beep()
}
