//go:build darwin

package real

import "os/exec"

// quickLook starts the system's quick preview of path, the way the 1.x app does: qlmanage shows its own window and
// the process is released, so the palette never waits for it.
func quickLook(path string) error {
	cmd := exec.Command("qlmanage", "-p", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
