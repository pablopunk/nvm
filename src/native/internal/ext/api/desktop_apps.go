package api

// App is an installed application. ID is the platform identifier (bundle id on macOS); show Name.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	FirstSeenMs int64  `json:"firstSeenMs,omitempty"`
}

// Apps lists and launches applications from the host index.
type Apps interface {
	// Frontmost names the app in front. ID is its bundle id (its window class on X11). Path is set when the installed
	// index lists the app, which is the path Icon takes; it is empty otherwise.
	Frontmost() (App, error)
	Launch(path string) error
	List() ([]App, error)
	Search(query string) ([]App, error)
	Icon(path string) (dataURL string, found bool, err error)
}
