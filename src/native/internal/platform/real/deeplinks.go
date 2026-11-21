package real

import "github.com/egoist/mygo"

// deepLinks registers URL schemes and receives their URLs through MyGo. On macOS a scheme must also be listed in
// urlSchemes of mygo.config.ts; MyGo registers it with the system from there.
type deepLinks struct{}

// Register claims scheme for the current user.
func (deepLinks) Register(scheme string) error {
	return mygo.App.RegisterURLScheme(scheme)
}

// OnOpen calls fn with each URL of a handled scheme.
func (deepLinks) OnOpen(fn func(rawURL string)) (off func()) {
	return mygo.App.OnOpenURL(fn)
}
