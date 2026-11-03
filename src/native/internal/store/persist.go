package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// saveLocked writes the document when its redacted content differs from the file on disk. A failed write
// keeps the change in memory and emits a SaveFailed notice. The caller holds c.mu.
func (c *Collection[T]) saveLocked() error {
	if c.readOnly {
		return ErrReadOnly
	}
	payload, omitted, err := c.encodeLocked()
	if err != nil {
		c.store.notify(Notice{Kind: SaveFailed, Path: c.path, Reason: "document could not be encoded"})
		return fmt.Errorf("store: encode %s: %w", c.name, err)
	}
	if len(omitted) > 0 {
		c.store.notify(Notice{Kind: SecretOmitted, Path: c.path, Reason: "secret field omitted: " + strings.Join(omitted, ", ")})
	}
	if !bytes.Equal(payload, c.onDisk) {
		if err := c.writeLocked(payload); err != nil {
			c.store.notify(Notice{Kind: SaveFailed, Path: c.path, Reason: err.Error()})
			return err
		}
	}
	c.dirty = false
	if len(omitted) > 0 {
		return fmt.Errorf("%w (%s)", ErrSecretInStore, c.name)
	}
	return nil
}

// writeLocked commits payload: it verifies the lease, replaces the file through a temporary file, and
// rotates the previous content into the backups. A rotation failure is reported but does not undo the
// committed write. The caller holds c.mu.
func (c *Collection[T]) writeLocked(payload []byte) error {
	if err := c.store.verifyLease(); err != nil {
		return err
	}
	previous := c.onDisk
	if err := c.replaceFile(c.tmpPath, c.path, payload); err != nil {
		return err
	}
	c.onDisk = payload
	if err := c.rotateBackups(previous); err != nil {
		c.store.notify(Notice{Kind: BackupFailed, Path: c.path, Reason: err.Error()})
	}
	c.store.savedHook(c.path)
	return nil
}

// replaceFile writes data to tmp, syncs it, and renames it over dest. A failure before the rename leaves
// dest untouched and removes tmp. The directory sync after the rename is best-effort, because some OSes
// refuse to sync folders.
func (c *Collection[T]) replaceFile(tmp, dest string, data []byte) error {
	files := c.store.files
	mode := c.store.cfg.fileMode
	if err := files.WriteFile(tmp, data, mode); err != nil {
		c.discard(tmp)
		return fmt.Errorf("store: write %s: %w", filepath.Base(tmp), err)
	}
	if err := files.Chmod(tmp, mode); err != nil {
		c.discard(tmp)
		return fmt.Errorf("store: set mode of %s: %w", filepath.Base(tmp), err)
	}
	if err := files.Sync(tmp); err != nil {
		c.discard(tmp)
		return fmt.Errorf("store: sync %s: %w", filepath.Base(tmp), err)
	}
	if err := files.Rename(tmp, dest); err != nil {
		c.discard(tmp)
		return fmt.Errorf("store: replace %s: %w", filepath.Base(dest), err)
	}
	_ = files.Sync(filepath.Dir(dest))
	return nil
}

// discard removes a temporary file after a failed write. Leftovers are harmless: loading deletes them.
func (c *Collection[T]) discard(tmp string) {
	_ = c.store.files.Remove(tmp)
}

// rotateBackups shifts name.json.bak.1 through .bak.N up by one and stores previous as .bak.1, so the
// newest good copies are always the lowest numbers. A copy past the window is dropped.
func (c *Collection[T]) rotateBackups(previous []byte) error {
	keep := c.store.cfg.keepBackups
	if keep == 0 || previous == nil {
		return nil
	}
	files := c.store.files
	_ = files.Remove(c.backupPath(keep + 1))
	for i := keep - 1; i >= 1; i-- {
		if _, err := files.Stat(c.backupPath(i)); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("store: stat backup %d: %w", i, err)
		}
		if err := files.Rename(c.backupPath(i), c.backupPath(i+1)); err != nil {
			return fmt.Errorf("store: shift backup %d: %w", i, err)
		}
	}
	return c.replaceFile(c.tmpPath+".bak", c.backupPath(1), previous)
}

// backupPath is the file name of backup copy i, where 1 is the newest.
func (c *Collection[T]) backupPath(i int) string {
	return fmt.Sprintf("%s.bak.%d", c.path, i)
}
