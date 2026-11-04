package api

// App is an installed application. ID is the platform identifier (bundle id on macOS); show Name.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	FirstSeenMs int64  `json:"firstSeenMs,omitempty"`
}

// QuitResult says what a quit request found and did to one app. WasRunning is false when the app had no live process,
// so there was nothing to quit. Quit is true when the request left the app with no live process. Killed is true when
// the app was force quit, or was killed after its grace period.
type QuitResult struct {
	WasRunning bool `json:"wasRunning"`
	Quit       bool `json:"quit"`
	Killed     bool `json:"killed"`
}

// MaxRunningPaths bounds one Running call: the apps a palette list asks about at once.
const MaxRunningPaths = 30

// Apps lists, launches and controls applications from the host index. Every path must name an app the index lists;
// any other path is refused, so an extension cannot act on an arbitrary binary.
type Apps interface {
	// Frontmost names the app in front. ID is its bundle id (its window class on X11). Path is set when the installed
	// index lists the app, which is the path Icon takes; it is empty otherwise.
	Frontmost() (App, error)
	Launch(path string) error
	// List returns the apps the host index lists, sorted by name, each with the time the host first saw it. The list
	// is capped at 500 apps.
	List() ([]App, error)
	Search(query string) ([]App, error)
	Icon(path string) (dataURL string, found bool, err error)
	// Running returns the paths among paths whose app has a live process, in request order. It asks at most
	// MaxRunningPaths paths. Paths the index does not list are never running.
	Running(paths []string) ([]string, error)
	// Quit asks the app to quit. On macOS the app is asked itself, so it may decline, for example to save first; the
	// result says whether it went within the grace period. Elsewhere its processes are signalled.
	Quit(path string) (QuitResult, error)
	// ForceQuit ends the app at once, without asking it.
	ForceQuit(path string) (QuitResult, error)
	// QuitAll asks every running app to quit except Nevermind and the desktop's own apps. It needs the system
	// declaration as well as desktop.apps. It returns the apps it asked.
	QuitAll() ([]App, error)
	// Hide hides the app's windows. Only macOS can; elsewhere the call fails.
	Hide(path string) error
	// UninstallReview finds what uninstalling the app at path would move to the Trash: the app and the data it left. The
	// host keeps the review under its Token until the review is used or is replaced by a newer one. Ready is false when
	// the platform cannot uninstall, or the app cannot be uninstalled, and Message says why.
	UninstallReview(path string) (UninstallReview, error)
	// UninstallTrash moves the items of a review, named by their IDs, to the Trash. Each item is checked again just before
	// it moves, and the app itself is asked to quit when it runs. Items the review did not list are ignored. It needs
	// the review's token, which is refused once the host has dropped the review.
	UninstallTrash(token string, ids []string) (UninstallResult, error)
}

// UninstallReview is the review of one app's uninstall. Items are the candidates the review lists, Selected by default
// as the host chose them.
type UninstallReview struct {
	Token   string          `json:"token,omitempty"`
	Ready   bool            `json:"ready"`
	Message string          `json:"message,omitempty"`
	Notes   []string        `json:"notes,omitempty"`
	Items   []UninstallItem `json:"items,omitempty"`
}

// UninstallItem is one candidate of an uninstall review. Kind is "app" for the application itself and "associated" for
// its data; Reason says why the review lists it.
type UninstallItem struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason,omitempty"`
	Selected bool   `json:"selected"`
}

// UninstallResult says what a Trash run moved. Status is "complete" when every selected item moved, "partial" when some
// did, and "failed" when none did. Untouched lists the selected items that stayed, with the reason for each.
type UninstallResult struct {
	Status    string               `json:"status"`
	Moved     []string             `json:"moved,omitempty"`
	Untouched []UninstallUntouched `json:"untouched,omitempty"`
	Notes     []string             `json:"notes,omitempty"`
}

// UninstallUntouched is a selected item a Trash run left in place, and why.
type UninstallUntouched struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
