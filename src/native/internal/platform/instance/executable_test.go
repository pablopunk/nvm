package instance

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type launchFixture struct {
	t                 *testing.T
	mount, other, bin string
	realExe, otherExe string
}

func newLaunchFixture(t *testing.T) launchFixture {
	t.Helper()
	root := t.TempDir()
	f := launchFixture{t: t, mount: filepath.Join(root, "tmp", ".mount_nevermind"), other: filepath.Join(root, "tmp", ".mount_t3code"), bin: filepath.Join(root, "bin")}
	for _, dir := range []string{filepath.Join(f.mount, "usr", "bin"), filepath.Join(f.other, "usr", "bin"), f.bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.realExe = filepath.Join(f.mount, "usr", "bin", "nevermind")
	f.otherExe = filepath.Join(f.other, "usr", "bin", "t3code")
	for _, p := range []string{f.realExe, f.otherExe} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f launchFixture) resolve(env map[string]string, exe, invoked string) string {
	f.t.Helper()
	got, err := resolveExecutable(func(k string) string { return env[k] }, func() (string, error) { return exe, nil }, filepath.Abs, exec.LookPath, invoked)
	if err != nil {
		f.t.Fatal(err)
	}
	return got
}

// The AppImage path is used only by the AppImage whose mount holds the running file.
func TestExecutableUsesAppImageOnlyFromItsOwnMount(t *testing.T) {
	f := newLaunchFixture(t)
	own := map[string]string{"APPIMAGE": "/home/me/Nevermind.AppImage", "APPDIR": f.mount}
	if got := f.resolve(own, f.realExe, f.realExe); got != "/home/me/Nevermind.AppImage" {
		t.Fatalf("own AppImage: %q", got)
	}
	foreign := map[string]string{"APPIMAGE": "/home/me/T3-Code.AppImage", "APPDIR": f.other}
	if got := f.resolve(foreign, f.realExe, f.realExe); got != f.realExe {
		t.Fatalf("another app's AppImage environment: %q, want the running executable", got)
	}
	if got := f.resolve(map[string]string{"APPIMAGE": "/home/me/T3-Code.AppImage"}, f.realExe, f.realExe); got != f.realExe {
		t.Fatalf("APPIMAGE without APPDIR: %q", got)
	}
	sibling := map[string]string{"APPIMAGE": "/x.AppImage", "APPDIR": f.mount + "-other"}
	if got := f.resolve(sibling, f.realExe, f.realExe); got != f.realExe {
		t.Fatalf("a directory that only shares a prefix: %q", got)
	}
}

// An install through a symlink keeps the symlink, so an update that replaces the file behind it stays bound.
func TestExecutableKeepsTheInstalledSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	f := newLaunchFixture(t)
	link := filepath.Join(f.bin, "nevermind-next")
	if err := os.Symlink(f.realExe, link); err != nil {
		t.Fatal(err)
	}
	if got := f.resolve(nil, f.realExe, link); got != link {
		t.Fatalf("started through the symlink: %q, want %q", got, link)
	}
	t.Setenv("PATH", f.bin)
	if got := f.resolve(nil, f.realExe, "nevermind-next"); got != link {
		t.Fatalf("started by name from PATH: %q, want %q", got, link)
	}
	if got := f.resolve(nil, f.realExe, f.otherExe); got != f.realExe {
		t.Fatalf("argv[0] that is another file: %q, want the running executable", got)
	}
	foreign := map[string]string{"APPIMAGE": "/home/me/T3-Code.AppImage", "APPDIR": f.other}
	if got := f.resolve(foreign, f.realExe, link); got != link {
		t.Fatalf("symlink under another app's environment: %q", got)
	}
}
