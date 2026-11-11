//go:build unix

package instance

import (
	"os/exec"
	"syscall"
)

// detach starts the child in its own session, so closing the terminal that ran the command does not stop it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
