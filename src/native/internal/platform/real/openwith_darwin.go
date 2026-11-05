//go:build darwin

package real

import (
	"os/exec"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macOpen is the system's open command, which opens a file with a named application.
const macOpen = "/usr/bin/open"

// openWith runs open -a, which opens the file with the application bundle at app.Path.
func openWith(path string, app platform.App) error {
	return exec.Command(macOpen, "-a", app.Path, path).Run()
}
