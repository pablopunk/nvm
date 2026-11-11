package instance

import (
	"fmt"
	"os"
	"os/exec"
)

// BackgroundFlag starts the app without showing the palette, as a launch for the action command line does.
const BackgroundFlag = "--background"

// StartBackground starts executable with BackgroundFlag, detached from the caller's session and terminal, and returns
// once it started. It does not wait for the app to be ready.
func StartBackground(executable string) error {
	cmd := exec.Command(executable, BackgroundFlag)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("instance: start %s: %w", executable, err)
	}
	return cmd.Process.Release()
}

// Executable is the path a binding or a launch should call: the AppImage file when the app runs from one, since its
// mounted path changes on every run, else the running executable.
func Executable() (string, error) {
	if image := os.Getenv("APPIMAGE"); image != "" {
		return image, nil
	}
	return os.Executable()
}
