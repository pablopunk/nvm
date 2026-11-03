package platform

// Notifier is the intent "tell the user something happened while the app is in the background", such
// as a timer finishing. Desktop notifications may be turned off by the user or the OS; that returns
// ErrDenied. Every OS supports it on the platforms the app ships to.
type Notifier interface {
	// Show displays a notification with a title and a body.
	Show(title, body string) error
}
