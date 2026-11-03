package apps

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// IconStore is the disk cache of icon PNGs, keyed by the names iconKey produces.
type IconStore interface {
	// Read returns the stored PNG for key. A missing key is ok=false and a nil error.
	Read(key string) (data []byte, ok bool, err error)
	// Write stores the PNG for key, replacing any earlier one.
	Write(key string, data []byte) error
}

// DiskIconStore keeps one PNG per key in Dir. Writes go to a temporary file that is renamed into place, so a crash
// never leaves a truncated icon behind.
type DiskIconStore struct {
	Files platform.FS
	Dir   string
}

const iconFileSuffix = ".png"

// Read returns the PNG stored under key.
func (d DiskIconStore) Read(key string) ([]byte, bool, error) {
	data, err := d.Files.ReadFile(filepath.Join(d.Dir, key+iconFileSuffix))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, platform.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Write stores data under key with the app's private file mode.
func (d DiskIconStore) Write(key string, data []byte) error {
	if err := d.Files.MkdirAll(d.Dir, 0o700); err != nil {
		return err
	}
	final := filepath.Join(d.Dir, key+iconFileSuffix)
	tmp := final + ".tmp"
	if err := d.Files.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return d.Files.Rename(tmp, final)
}
