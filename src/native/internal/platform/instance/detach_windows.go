//go:build windows

package instance

import (
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008

// detach starts the child without the caller's console.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP}
}
