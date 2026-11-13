package codemode

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// stalePrograms is how old a staged program folder must be before the next run removes it. Every run removes its own
// folder; this only clears what a crash left behind, and a live run is never that old.
const stalePrograms = time.Hour

// stagedProgram is the folder one run writes its source into, owned by this run alone.
type stagedProgram struct {
	fs   platform.FS
	dir  string
	path string
}

// stage writes the program source to a new private folder under root. The folder name carries the time, so a later run
// can tell a leftover from a live run.
func stage(fsys platform.FS, clock platform.Clock, root, source string) (*stagedProgram, error) {
	sweep(fsys, clock.Now(), root)
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, fmt.Errorf("cannot name the program folder: %w", err)
	}
	dir := filepath.Join(root, fmt.Sprintf("run-%d-%s", clock.Now().Unix(), hex.EncodeToString(token[:])))
	if err := fsys.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cannot create the program folder: %w", err)
	}
	staged := &stagedProgram{fs: fsys, dir: dir, path: filepath.Join(dir, "main.go")}
	if err := fsys.WriteFile(staged.path, []byte(source), 0o600); err != nil {
		staged.remove()
		return nil, fmt.Errorf("cannot stage the program: %w", err)
	}
	return staged, nil
}

// remove deletes the folder. It runs on every path out of a run.
func (s *stagedProgram) remove() {
	_ = s.fs.Remove(s.path)
	_ = s.fs.Remove(s.dir)
}

// sweep removes folders older than stalePrograms.
func sweep(fsys platform.FS, now time.Time, root string) {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if started, ok := startedAt(entry.Name()); ok && now.Sub(started) > stalePrograms {
			dir := filepath.Join(root, entry.Name())
			removeFolder(fsys, dir)
		}
	}
}

func removeFolder(fsys platform.FS, dir string) {
	if files, err := fsys.ReadDir(dir); err == nil {
		for _, file := range files {
			if !file.IsDir() {
				_ = fsys.Remove(filepath.Join(dir, file.Name()))
			}
		}
	}
	_ = fsys.Remove(dir)
}

// startedAt reads the time out of a run folder's name.
func startedAt(name string) (time.Time, bool) {
	rest, found := strings.CutPrefix(name, "run-")
	if !found {
		return time.Time{}, false
	}
	seconds, _, found := strings.Cut(rest, "-")
	if !found {
		return time.Time{}, false
	}
	unix, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}
