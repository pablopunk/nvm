package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ErrRunning reports that another process already answers on the socket.
var ErrRunning = errors.New("instance: another instance is listening")

// maxSocketPath keeps the path under the about 104 bytes a Unix socket address allows.
const maxSocketPath = 100

// Place names where the socket of one app identity lives: the runtime folders to try, in order, and the user.
type Place struct {
	// Dirs are the candidate base folders, such as $XDG_RUNTIME_DIR then the temporary folder. Empty entries are skipped.
	Dirs []string
	// UID names the private folder, so two users never share one.
	UID int
	// Key identifies the app identity, such as its data folder, so a development and an installed build keep apart.
	Key string
}

// DefaultPlace is the place of the current user: $XDG_RUNTIME_DIR when set, else the temporary folder.
func DefaultPlace(key string) Place {
	return Place{Dirs: []string{os.Getenv("XDG_RUNTIME_DIR"), os.TempDir(), "/tmp"}, UID: os.Getuid(), Key: key}
}

// SocketPath is the socket's path: <base>/nevermind-<uid>/actions-<hash>.sock in the first base folder short enough.
func (p Place) SocketPath() string {
	sum := sha256.Sum256([]byte(p.Key))
	rel := filepath.Join("nevermind-"+strconv.Itoa(p.UID), "actions-"+hex.EncodeToString(sum[:6])+".sock")
	for _, dir := range p.Dirs {
		if dir != "" && len(filepath.Join(dir, rel)) < maxSocketPath {
			return filepath.Join(dir, rel)
		}
	}
	return filepath.Join("/tmp", rel)
}

// Listen opens the socket for the running instance. The folder is created private to the user and refused when
// another user owns it or others can enter it. A socket nobody answers on is a leftover of a crash and is replaced; a
// live one returns ErrRunning.
func (p Place) Listen() (net.Listener, error) {
	path := p.SocketPath()
	if err := ensurePrivateDir(filepath.Dir(path), p.UID); err != nil {
		return nil, err
	}
	if conn, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		conn.Close()
		return nil, ErrRunning
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("instance: listen: %w", err)
	}
	return ln, nil
}

// Dial connects to the running instance within timeout. A missing or dead socket is an error the caller reads as "no
// instance is running".
func (p Place) Dial(timeout time.Duration) (net.Conn, error) {
	path := p.SocketPath()
	if err := checkPrivateDir(filepath.Dir(path), p.UID); err != nil {
		return nil, err
	}
	return net.DialTimeout("unix", path, timeout)
}

// ensurePrivateDir creates dir with mode 0700 when missing, then checks it as checkPrivateDir does.
func ensurePrivateDir(dir string, uid int) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("instance: create %s: %w", dir, err)
	}
	return checkPrivateDir(dir, uid)
}

// checkPrivateDir refuses a folder that is a link, is not a folder, belongs to another user, or lets others in, so no
// other account can plant or read the socket.
func checkPrivateDir(dir string, uid int) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("instance: %s is not a private folder", dir)
	}
	if !ownedPrivately(info, uid) {
		return fmt.Errorf("instance: %s is shared with other users", dir)
	}
	return nil
}
