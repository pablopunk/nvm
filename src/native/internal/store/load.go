package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// maxQuarantineAttempts bounds the search for a free quarantine name within one second.
const maxQuarantineAttempts = 100

// loadLocked reads the document from disk into memory. The caller holds c.mu. It writes only to keep a
// copy of a file it is about to migrate.
func (c *Collection[T]) loadLocked() {
	c.loaded = true
	c.removeLeftovers()
	raw, err := c.store.files.ReadFile(c.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		c.value = c.defaults()
		return
	case err != nil:
		c.failLoad(fmt.Errorf("store: read %s: %w", c.name, err), "file could not be read; opened read-only")
		return
	}
	stored, reason := decodeEnvelope(raw)
	if reason != "" {
		c.quarantineLocked(reason)
		return
	}
	switch {
	case stored.version > c.version:
		c.loadNewerLocked(raw, stored)
	case stored.version < c.version:
		c.loadMigratedLocked(raw, stored)
	default:
		c.loadCurrentLocked(raw, stored)
	}
}

// loadCurrentLocked decodes a file at the current schema version.
func (c *Collection[T]) loadCurrentLocked(raw []byte, stored storedEnvelope) {
	value, err := decodeDocument(c.defaults, stored.data)
	if err != nil {
		c.quarantineLocked("data does not match the document type")
		return
	}
	c.value = value
	c.onDisk = raw
}

// loadNewerLocked opens a file written by a newer build read-only, so its fields are shown but never
// overwritten by this build.
func (c *Collection[T]) loadNewerLocked(raw []byte, stored storedEnvelope) {
	c.readOnly = true
	c.onDisk = raw
	reason := fmt.Sprintf("written by a newer build (schema %d, this build reads %d); opened read-only", stored.version, c.version)
	value, err := decodeDocument(c.defaults, stored.data)
	if err != nil {
		c.value = c.defaults()
		reason = fmt.Sprintf("written by a newer build (schema %d); its data could not be read; opened read-only", stored.version)
	} else {
		c.value = value
	}
	c.store.notify(Notice{Kind: NewerVersion, Path: c.path, Reason: reason})
}

// loadMigratedLocked runs the migration chain on an older file, keeps the original bytes as
// name.json.premigrate-vN, and saves the migrated document at once.
func (c *Collection[T]) loadMigratedLocked(raw []byte, stored storedEnvelope) {
	data, err := runMigrations(c.migrations, stored.version, c.version, stored.data)
	if err != nil {
		c.failLoad(err, "migration failed; the file was left as it is; opened read-only")
		return
	}
	value, err := decodeDocument(c.defaults, data)
	if err != nil {
		c.failLoad(fmt.Errorf("%w: %s: migrated data does not match the document type", ErrMigration, c.name), "migrated data does not match the document; opened read-only")
		return
	}
	if err := c.keepPreMigrationCopy(raw, stored.version); err != nil {
		c.failLoad(err, "could not back up the file before migrating; opened read-only")
		return
	}
	c.value = value
	c.onDisk = raw
	c.dirty = true
	if err := c.saveLocked(); err != nil {
		c.loadErr = err
	}
}

// keepPreMigrationCopy writes the original bytes next to the document, once per source version. An existing
// file is kept as it is: it holds the first state a migration saw. A folder in the way is an error, because
// migrating without a copy could lose the original.
func (c *Collection[T]) keepPreMigrationCopy(raw []byte, fromVersion int) error {
	path := filepath.Join(c.store.dir, fmt.Sprintf("%s.premigrate-v%d", c.name, fromVersion))
	info, err := c.store.files.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return fmt.Errorf("store: %s is a folder, so no pre-migration copy can be kept", filepath.Base(path))
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("store: stat pre-migration copy: %w", err)
	}
	return c.replaceFile(c.tmpPath+".premigrate", path, raw)
}

// failLoad makes the document read-only with its defaults. The file stays on disk untouched.
func (c *Collection[T]) failLoad(err error, reason string) {
	c.value = c.defaults()
	c.readOnly = true
	c.loadErr = err
	c.store.notify(Notice{Kind: LoadFailed, Path: c.path, Reason: reason})
}

// quarantineLocked moves a corrupt file to name.json.corrupt-<unix> (never deleting it) and starts from the
// defaults. If the file cannot be moved, the document opens read-only so nothing is overwritten.
func (c *Collection[T]) quarantineLocked(reason string) {
	target, err := c.freeQuarantinePath()
	if err == nil {
		err = c.store.files.Rename(c.path, target)
	}
	if err != nil {
		c.failLoad(fmt.Errorf("store: quarantine %s: %w", c.name, err), "corrupt file could not be moved aside; opened read-only")
		return
	}
	c.value = c.defaults()
	c.onDisk = nil
	c.store.notify(Notice{Kind: Quarantined, Path: target, Reason: reason})
}

// freeQuarantinePath picks a quarantine name that does not exist yet.
func (c *Collection[T]) freeQuarantinePath() (string, error) {
	base := fmt.Sprintf("%s.corrupt-%d", c.path, c.store.clock.Now().Unix())
	for attempt := range maxQuarantineAttempts {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		_, err := c.store.files.Stat(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("store: no free quarantine name for %s", c.name)
}

// removeLeftovers deletes temporary files that a crashed write left behind. They never hold committed
// data, because the rename that commits a write is its last step.
func (c *Collection[T]) removeLeftovers() {
	files := c.store.files
	_ = files.Remove(c.tmpPath)
	_ = files.Remove(c.tmpPath + ".bak")
	_ = files.Remove(c.tmpPath + ".premigrate")
}

// decodeDocument decodes stored data over a fresh defaults value, so fields missing from the file keep
// their defaults.
func decodeDocument[T any](defaults func() T, data []byte) (T, error) {
	value := defaults()
	if err := json.Unmarshal(data, &value); err != nil {
		return defaults(), err
	}
	return value, nil
}
