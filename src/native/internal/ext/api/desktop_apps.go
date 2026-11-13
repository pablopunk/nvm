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
	// List returns the apps the host index lists, with the time each was first seen.
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
	// Hide hides the app's windows. Platform: macOS only (see PlatformOperations). An extension whose source calls it
	// is not run on another OS.
	Hide(path string) error
	// UninstallReview reviews what uninstalling the app at path would move. Token names the review for UninstallTrash.
	UninstallReview(path string) (UninstallReview, error)
	// UninstallTrash moves the review's items with these IDs to the Trash, checking each one again first.
	UninstallTrash(token string, ids []string) (UninstallResult, error)
}

// UninstallReview is an uninstall review: its candidates and notes about them.
type UninstallReview struct {
	Token   string          `json:"token,omitempty"`
	Ready   bool            `json:"ready"`
	Message string          `json:"message,omitempty"`
	Notes   []string        `json:"notes,omitempty"`
	Items   []UninstallItem `json:"items,omitempty"`
}

// UninstallItem is a candidate of an uninstall review.
type UninstallItem struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason,omitempty"`
	Selected bool   `json:"selected"`
}

// UninstallResult says what a Trash run moved; Status is complete, partial or failed.
type UninstallResult struct {
	Status      string               `json:"status"`
	TrashFolder string               `json:"trashFolder,omitempty"`
	Moved       []string             `json:"moved,omitempty"`
	Untouched   []UninstallUntouched `json:"untouched,omitempty"`
}

// UninstallUntouched is a selected item a Trash run left in place.
type UninstallUntouched struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
