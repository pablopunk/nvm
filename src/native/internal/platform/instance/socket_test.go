//go:build unix

package instance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "nvi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestSocketPathIsPerUserAndPerKey(t *testing.T) {
	a := Place{Dirs: []string{"/run/user/1000"}, UID: 1000, Key: "dev"}
	b := Place{Dirs: []string{"/run/user/1000"}, UID: 1000, Key: "packaged"}
	if a.SocketPath() == b.SocketPath() {
		t.Fatal("two identities share a socket")
	}
	if !strings.HasPrefix(a.SocketPath(), "/run/user/1000/nevermind-1000/actions-") {
		t.Fatalf("path %q", a.SocketPath())
	}
}

func TestSocketPathSkipsLongAndEmptyDirs(t *testing.T) {
	p := Place{Dirs: []string{"", "/" + strings.Repeat("x", 120), "/tmp"}, UID: 7, Key: "k"}
	if !strings.HasPrefix(p.SocketPath(), "/tmp/nevermind-7/") {
		t.Fatalf("path %q", p.SocketPath())
	}
}

func TestListenDialRoundTripAndSecondListenIsRefused(t *testing.T) {
	p := Place{Dirs: []string{shortTempDir(t)}, UID: os.Getuid(), Key: "k"}
	ln, err := p.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Write([]byte("ok"))
			conn.Close()
		}
	}()
	conn, err := p.Dial(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2)
	if _, err := conn.Read(buf); err != nil || string(buf) != "ok" {
		t.Fatalf("read %q %v", buf, err)
	}
	conn.Close()
	if _, err := p.Listen(); !errors.Is(err, ErrRunning) {
		t.Fatalf("second listen: %v", err)
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	p := Place{Dirs: []string{shortTempDir(t)}, UID: os.Getuid(), Key: "k"}
	ln, err := p.Listen()
	if err != nil {
		t.Fatal(err)
	}
	// A crashed instance leaves the file behind.
	ln.(interface{ SetUnlinkOnClose(bool) }).SetUnlinkOnClose(false)
	ln.Close()
	if _, err := os.Stat(p.SocketPath()); err != nil {
		t.Fatalf("stale socket missing: %v", err)
	}
	again, err := p.Listen()
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
}

func TestSharedFolderIsRefused(t *testing.T) {
	base := shortTempDir(t)
	p := Place{Dirs: []string{base}, UID: os.Getuid(), Key: "k"}
	dir := filepath.Dir(p.SocketPath())
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o777)
	if _, err := p.Listen(); err == nil {
		t.Fatal("listened in a folder others can enter")
	}
	if _, err := p.Dial(time.Second); err == nil {
		t.Fatal("dialed through a folder others can enter")
	}
}

func TestSymlinkedFolderIsRefused(t *testing.T) {
	base := shortTempDir(t)
	p := Place{Dirs: []string{base}, UID: os.Getuid(), Key: "k"}
	target := filepath.Join(base, "elsewhere")
	os.Mkdir(target, 0o700)
	if err := os.Symlink(target, filepath.Dir(p.SocketPath())); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Listen(); err == nil {
		t.Fatal("listened through a symlink")
	}
}

func TestOtherUIDIsRefused(t *testing.T) {
	p := Place{Dirs: []string{shortTempDir(t)}, UID: os.Getuid() + 1, Key: "k"}
	if _, err := p.Listen(); err == nil {
		t.Fatal("listened in a folder owned by another user")
	}
}
