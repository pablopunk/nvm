//go:build linux

package real

import (
	"os/exec"
	"syscall"
)

// dieWithParent makes the kernel kill the child when the app's thread exits, so a watcher never outlives the app.
func dieWithParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
