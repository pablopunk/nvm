package instance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Executable is the path a binding or a launch should call. The AppImage file when this process runs from that
// AppImage's own mount, since the mounted path changes on every run. APPIMAGE alone is not trusted: another AppImage's
// environment (an app that starts Nevermind) carries its own APPIMAGE and APPDIR. Otherwise the way this program was
// started when that is a path to the running file, such as a symlink in ~/.local/bin, else the real executable.
func Executable() (string, error) {
	return resolveExecutable(os.Getenv, os.Executable, filepath.Abs, exec.LookPath, os.Args[0])
}

func resolveExecutable(getenv func(string) string, executable func() (string, error), abs func(string) (string, error), lookPath func(string) (string, error), invoked string) (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	if image := getenv("APPIMAGE"); image != "" && insideDir(exe, getenv("APPDIR")) {
		return image, nil
	}
	if path := invokedPath(invoked, abs, lookPath); path != "" && sameFile(path, exe) {
		return path, nil
	}
	return exe, nil
}

// invokedPath is the absolute path argv[0] names: itself when it has a directory, else the file it finds on PATH.
func invokedPath(invoked string, abs func(string) (string, error), lookPath func(string) (string, error)) string {
	if invoked == "" {
		return ""
	}
	if !strings.ContainsRune(invoked, filepath.Separator) {
		found, err := lookPath(invoked)
		if err != nil {
			return ""
		}
		invoked = found
	}
	path, err := abs(invoked)
	if err != nil {
		return ""
	}
	return path
}

// insideDir reports whether path is dir or below it, comparing real paths. An empty dir contains nothing.
func insideDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	real := func(p string) string {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return resolved
		}
		return filepath.Clean(p)
	}
	rel, err := filepath.Rel(real(dir), real(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}
