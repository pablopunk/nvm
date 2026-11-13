package files

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// MaxSnapshotBytes caps the encoded snapshot the index saves and the file it loads. A snapshot costs about 40 bytes per
// file plus its name and folder bytes, so the cap holds several hundred thousand files, far past the index's own entry
// limit; a larger file is treated as damage rather than read into memory.
const MaxSnapshotBytes = 64 << 20

// ErrSnapshotTooLarge is returned when a snapshot is over MaxSnapshotBytes. The index then keeps running from its scan.
var ErrSnapshotTooLarge = errors.New("files: the index snapshot is larger than the size cap")

// CappedFilePersistence is FilePersistence with a size sanity cap on both directions.
type CappedFilePersistence struct {
	FS   platform.FS
	Path string
}

// Load reads the snapshot unless the file is over the cap; a missing file reports fs.ErrNotExist as usual.
func (p CappedFilePersistence) Load() ([]byte, error) {
	info, err := p.FS.Stat(p.Path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxSnapshotBytes {
		return nil, ErrSnapshotTooLarge
	}
	return FilePersistence(p).Load()
}

// Save writes the snapshot atomically, as FilePersistence does, after checking the cap.
func (p CappedFilePersistence) Save(data []byte) error {
	if len(data) > MaxSnapshotBytes {
		return ErrSnapshotTooLarge
	}
	return FilePersistence(p).Save(data)
}
