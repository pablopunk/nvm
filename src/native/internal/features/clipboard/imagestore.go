package clipboard

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrInvalidImageName refuses a name or hash that is not one the store creates. The check keeps names from
// the saved history from reaching outside the store's folder.
var ErrInvalidImageName = errors.New("clipboard: not a name the image store creates")

// imageNamePattern matches the only file names the store creates: a content hash and .png.
var imageNamePattern = regexp.MustCompile(`^[0-9a-f]{40}\.png$`)

// hashPattern matches a content hash.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ImageStoreDeps are the collaborators of an ImageStore. Dir is the folder that holds only store files.
type ImageStoreDeps struct {
	Files platform.FS
	Clock platform.Clock
	Dir   string
	Log   *logx.Logger
}

// ImageStore saves clipboard images as PNG files under one folder. Files are named after their content
// hash, written with mode 0600, and only the garbage collector removes them. It is safe for concurrent use.
type ImageStore struct {
	deps ImageStoreDeps

	mu       sync.Mutex
	inflight *gcCall
	joined   int
}

// NewImageStore returns a store over deps.Dir.
func NewImageStore(deps ImageStoreDeps) *ImageStore {
	return &ImageStore{deps: deps}
}

// ImageName returns the file name of an image with the given content hash.
func ImageName(hash string) string {
	return hash + ".png"
}

// Save writes png under the name of hash and returns the name. A file already stored under that name is kept:
// the name is the content, so the stored picture is the same one. The write goes through a temporary file
// and a rename, so a crash never leaves a half-written image under a real name.
func (s *ImageStore) Save(hash string, png []byte) (string, error) {
	if !hashPattern.MatchString(hash) {
		return "", ErrInvalidImageName
	}
	name := ImageName(hash)
	target := filepath.Join(s.deps.Dir, name)
	if s.isRegularFile(target) {
		return name, nil
	}
	if err := s.deps.Files.MkdirAll(s.deps.Dir, 0o700); err != nil {
		return "", fmt.Errorf("clipboard: create image folder: %w", err)
	}
	temporary := target + ".tmp"
	if err := s.deps.Files.WriteFile(temporary, png, 0o600); err != nil {
		return "", fmt.Errorf("clipboard: write image: %w", err)
	}
	if err := s.deps.Files.Rename(temporary, target); err != nil {
		_ = s.deps.Files.Remove(temporary)
		return "", fmt.Errorf("clipboard: place image: %w", err)
	}
	return name, nil
}

// Read returns the PNG bytes of a stored image. A symbolic link or any other non-file in its place is an
// error, never a read through the link.
func (s *ImageStore) Read(name string) ([]byte, error) {
	path, err := s.path(name)
	if err != nil {
		return nil, err
	}
	if !s.isRegularFile(path) {
		return nil, fmt.Errorf("clipboard: image %s is missing: %w", name, fs.ErrNotExist)
	}
	return s.deps.Files.ReadFile(path)
}

// path returns the full path of a stored image after checking its name.
func (s *ImageStore) path(name string) (string, error) {
	if !imageNamePattern.MatchString(name) {
		return "", ErrInvalidImageName
	}
	return filepath.Join(s.deps.Dir, name), nil
}

// isRegularFile reports whether path is a regular file, without following a symbolic link.
func (s *ImageStore) isRegularFile(path string) bool {
	info, err := s.deps.Files.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}
