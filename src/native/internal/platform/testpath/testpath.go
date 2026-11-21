// Package testpath gives tests absolute paths that the running OS accepts. A POSIX literal such as "/home/me" is
// rooted but has no volume on Windows, so filepath.IsAbs rejects it; Abs builds the same location under the root
// of the volume the test runs on, so one expectation reads the same on every OS.
package testpath

import (
	"os"
	"path/filepath"
)

// Root returns the root of the volume the test runs on: "/" on Unix and the drive root of the working directory
// on Windows, such as `C:\`.
func Root() string {
	wd, err := os.Getwd()
	if err != nil {
		return string(filepath.Separator)
	}
	return filepath.VolumeName(wd) + string(filepath.Separator)
}

// Abs joins elems below Root. Abs("home", "me") is /home/me on Unix and C:\home\me on Windows.
func Abs(elems ...string) string {
	return filepath.Join(append([]string{Root()}, elems...)...)
}
