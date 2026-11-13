package manager

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// supportedHere reports whether the extension can run on this OS: every OS-specific operation its source calls exists
// here. Built-ins are compiled for the OS they run on. An unknown OS (a test without one) restricts nothing.
func (m *Manager) supportedHere(e *entry) bool {
	return e.builtin || m.ports.OS == "" || e.platforms.Includes(m.ports.OS)
}

// UnsupportedPlatformError is the refusal to run an extension on an OS that lacks an operation its source calls. It is
// not a failure of the extension: it stays enabled for the OS it is for, so the message says where it works.
type UnsupportedPlatformError struct {
	Title     string
	Platforms api.Platforms
}

func (e *UnsupportedPlatformError) Error() string {
	if len(e.Platforms) == 0 {
		return fmt.Sprintf("%s does not work on any OS", e.Title)
	}
	return fmt.Sprintf("%s only works on %s", e.Title, e.Platforms.Names())
}
