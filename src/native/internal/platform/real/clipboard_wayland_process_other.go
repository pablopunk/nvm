//go:build !linux

package real

import "os/exec"

// dieWithParent does nothing off Linux: the Wayland feed only runs there.
func dieWithParent(*exec.Cmd) {}
