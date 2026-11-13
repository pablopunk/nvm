package logx

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
)

// logFileMode keeps logs readable only by the user: they can name paths and errors.
const logFileMode fs.FileMode = 0o600

// joinPath is the native path of a file inside the log directory.
func joinPath(dir, name string) string {
	return filepath.Join(dir, name)
}

// rotatedPath names the i-th older copy: nvm.log.1 is the newest rotated file.
func rotatedPath(active string, i int) string {
	return active + "." + strconv.Itoa(i)
}

// load reads the active file so that appends continue it and its size is known.
func (c *core) load() error {
	if c.files == nil {
		return nil
	}
	data, err := c.files.ReadFile(c.activePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	c.active = data
	return nil
}

// appendBatch appends batch to the active file, rotating first when the result would exceed MaxBytes.
// The file is replaced through a temporary sibling and a rename, so a crash leaves the old content or
// the new content, never a cut-off line.
func (c *core) appendBatch(batch []byte) error {
	if len(c.active) > 0 && int64(len(c.active)+len(batch)) > c.opts.MaxBytes {
		if err := c.rotate(); err != nil {
			return err
		}
	}
	next := append(c.active, batch...)
	temporary := c.activePath + ".tmp"
	if err := c.files.WriteFile(temporary, next, logFileMode); err != nil {
		return err
	}
	if err := c.files.Rename(temporary, c.activePath); err != nil {
		return err
	}
	c.active = next
	return nil
}

// rotate shifts nvm.log.N-2 to nvm.log.N-1 and so on, moves the active file to nvm.log.1 and drops the
// oldest copy, so at most MaxFiles files exist including the active one. With MaxFiles of one there are
// no copies: the active file is simply replaced.
func (c *core) rotate() error {
	if c.opts.MaxFiles <= 1 {
		c.active = nil
		return nil
	}
	if err := c.remove(rotatedPath(c.activePath, c.opts.MaxFiles-1)); err != nil {
		return err
	}
	for i := c.opts.MaxFiles - 2; i >= 1; i-- {
		if err := c.rename(rotatedPath(c.activePath, i), rotatedPath(c.activePath, i+1)); err != nil {
			return err
		}
	}
	if err := c.rename(c.activePath, rotatedPath(c.activePath, 1)); err != nil {
		return err
	}
	c.active = nil
	return nil
}

// rename moves a file, treating a missing source as already done: a copy that was never written is
// not an error.
func (c *core) rename(from, to string) error {
	err := c.files.Rename(from, to)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (c *core) remove(name string) error {
	err := c.files.Remove(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
