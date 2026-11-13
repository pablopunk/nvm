//go:build windows

package real

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type osRelease struct{}

func newOSRelease() osRelease { return osRelease{} }

func (osRelease) KernelRelease() (string, error) {
	version := windows.RtlGetVersion()
	if version.MajorVersion == 0 {
		return "", fmt.Errorf("Windows kernel version unavailable")
	}
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber), nil
}
