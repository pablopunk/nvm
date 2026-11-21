//go:build linux

package real

import (
	"syscall"
)

type osRelease struct{}

func newOSRelease() osRelease { return osRelease{} }

func (osRelease) KernelRelease() (string, error) {
	var info syscall.Utsname
	if err := syscall.Uname(&info); err != nil {
		return "", err
	}
	return charsToString(info.Release[:]), nil
}

func charsToString(chars []int8) string {
	bytes := make([]byte, 0, len(chars))
	for _, char := range chars {
		if char == 0 {
			break
		}
		bytes = append(bytes, byte(char))
	}
	return string(bytes)
}
