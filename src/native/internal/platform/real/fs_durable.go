package real

import (
	"io/fs"
	"os"
)

// CreateExclusive writes a new file and fails with an error matching fs.ErrExist when the name is taken.
func (osFS) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// Sync flushes a file or folder to stable storage. It opens read-write when it can and falls back to a
// read-only handle, which is enough for fsync on POSIX folders and read-only files.
func (osFS) Sync(name string) error {
	file, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		file, err = os.Open(name)
		if err != nil {
			return err
		}
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
