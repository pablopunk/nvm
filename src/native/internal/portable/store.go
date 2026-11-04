// Package portable reads and writes config.json, the file of portable settings that 1.x and 2.0 share. Its format is
// 1.x's: version 1, a settings object, a shortcuts object and the sections 2.0 does not use, which it keeps as they are.
package portable

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// fileMode is the mode of config.json. It holds settings the user may not want other accounts to read.
const fileMode fs.FileMode = 0o600

// folderMode is the mode of the shared folder when this build creates it.
const folderMode fs.FileMode = 0o755

// ErrDanglingLink means config.json is a symbolic link to a file that does not exist. Writing would replace the link,
// so the file is left alone.
var ErrDanglingLink = errors.New("config.json is a symbolic link to a missing file")

// Store reads and writes config.json at one path. Each Load reads the file, so a change another app made is seen at
// once. Each Update rereads the file, applies its change to that copy and writes the result, so it never overwrites
// what another app wrote in between. A file that cannot be read, is invalid or is newer than this build is never
// written.
type Store struct {
	files platform.FS
	path  string
	mu    sync.Mutex
}

// Open returns the store of the config file at path.
func Open(files platform.FS, path string) *Store {
	return &Store{files: files, path: path}
}

// Path is the file the store reads and writes.
func (s *Store) Path() string {
	return s.path
}

// Load returns the document on disk. A missing file is an empty document, with no error. An unreadable or invalid
// file returns an empty document and the reason.
func (s *Store) Load() (Document, error) {
	doc, _, err := s.read()
	return doc, err
}

// Update applies change to the document on disk and writes it when change altered it. The file is written with mode
// 0600 through a temporary file and a rename, and an existing symbolic link keeps pointing where it did.
func (s *Store) Update(change func(Document)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, data, err := s.read()
	if err != nil {
		return err
	}
	before, err := doc.encode()
	if err != nil {
		return err
	}
	change(doc)
	after, err := doc.encode()
	if err != nil {
		return err
	}
	if data != nil && bytes.Equal(before, after) {
		return nil
	}
	return s.write(after)
}

func (s *Store) read() (Document, []byte, error) {
	target, err := s.resolve()
	if errors.Is(err, fs.ErrNotExist) {
		return newDocument(), nil, nil
	}
	if err != nil {
		return newDocument(), nil, err
	}
	data, err := s.files.ReadFile(target)
	if err != nil {
		return newDocument(), nil, fmt.Errorf("config.json could not be read: %w", err)
	}
	doc, err := parseDocument(data)
	if err != nil {
		return newDocument(), nil, err
	}
	return doc, data, nil
}

// resolve returns the file that holds the content: the link's target when the path is a link, else the path itself.
// An absent file returns the path with fs.ErrNotExist. A link whose target is gone returns ErrDanglingLink.
func (s *Store) resolve() (string, error) {
	target, err := s.files.EvalSymlinks(s.path)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if s.isLink() {
		return "", ErrDanglingLink
	}
	return s.path, fs.ErrNotExist
}

func (s *Store) isLink() bool {
	info, err := s.files.Lstat(s.path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

func parseDocument(data []byte) (Document, error) {
	obj, err := decodeDocument(data)
	if err != nil {
		return newDocument(), fmt.Errorf("config.json is not valid JSON: %w", err)
	}
	if err := validateDocument(obj); err != nil {
		return newDocument(), err
	}
	return Document{root: obj}, nil
}

func (s *Store) write(data []byte) error {
	target, err := s.resolve()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(target)
	if err := s.files.MkdirAll(dir, folderMode); err != nil {
		return fmt.Errorf("config.json folder could not be created: %w", err)
	}
	temporary := filepath.Join(dir, "."+filepath.Base(target)+"."+randomSuffix()+".tmp")
	if err := s.writeTemporary(temporary, data); err != nil {
		_ = s.files.Remove(temporary)
		return err
	}
	if err := s.files.Rename(temporary, target); err != nil {
		_ = s.files.Remove(temporary)
		return fmt.Errorf("config.json could not be replaced: %w", err)
	}
	_ = s.files.Sync(dir)
	return nil
}

func (s *Store) writeTemporary(name string, data []byte) error {
	if err := s.files.WriteFile(name, data, fileMode); err != nil {
		return fmt.Errorf("config.json could not be written: %w", err)
	}
	if err := s.files.Chmod(name, fileMode); err != nil {
		return fmt.Errorf("config.json mode could not be set: %w", err)
	}
	if err := s.files.Sync(name); err != nil {
		return fmt.Errorf("config.json could not be synced: %w", err)
	}
	return nil
}

func randomSuffix() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
