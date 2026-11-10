// Package install writes and removes extension folders so that a reader never sees half of one. An install
// stages the files in a hidden folder, loads them as a bundle, and only then renames the folder into place.
// Replaced and removed folders are renamed to a hidden trash name first and deleted last, so a crash leaves
// a hidden leftover that Sweep removes, never a visible half-written extension.
//
// Layer: ext. The package imports bundle, platform and the standard library only.
package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	stagingPrefix = ".staging-"
	trashPrefix   = ".trash-"
	dirMode       = 0o700
	fileMode      = 0o600
)

// Install writes files as the extension id under root. The files must include extension.json and main.go,
// and the manifest must load cleanly with exactly this id. Cleanup errors can follow publication;
// callers needing that distinction use InstallContextResult.
func Install(files platform.FS, clock platform.Clock, root, id string, contents map[string][]byte) error {
	return InstallContext(context.Background(), files, clock, root, id, contents)
}

// InstallContext stages a candidate, then checks cancellation immediately before the first publication
// rename. Cancellation after that admission does not roll back an admitted installation.
func InstallContext(ctx context.Context, files platform.FS, clock platform.Clock, root, id string, contents map[string][]byte) error {
	_, err := InstallContextResult(ctx, files, clock, root, id, contents)
	return err
}

// InstallContextResult also reports whether the candidate became visible, even if later cleanup fails.
func InstallContextResult(ctx context.Context, files platform.FS, clock platform.Clock, root, id string, contents map[string][]byte) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	staging, _, err := Stage(files, clock, root, id, contents)
	if err != nil {
		return false, err
	}
	return Publish(ctx, files, clock, root, id, staging)
}

// Publish admits an already staged candidate. The caller owns the staging folder until this call,
// allowing conditional installers to revalidate the target after all staging I/O has completed.
// A cancelled publication discards staging; the result reports whether the candidate became visible.
func Publish(ctx context.Context, files platform.FS, clock platform.Clock, root, id, staging string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errors.Join(err, Discard(files, staging))
	}
	return swapIn(files, clock, root, id, staging)
}

// Stage writes files into a hidden staging folder and checks that they load as id. It returns the staging folder
// and its loaded bundle. The caller publishes it with Install's swap or removes it with Discard. Nothing visible changes.
func Stage(files platform.FS, clock platform.Clock, root, id string, contents map[string][]byte) (string, bundle.Bundle, error) {
	nonce := strconv.FormatInt(clock.Now().UnixNano(), 36)
	staging := filepath.Join(root, stagingPrefix+id+"-"+nonce)
	if err := writeFolder(files, staging, contents); err != nil {
		_ = removeTree(files, staging)
		return "", bundle.Bundle{}, err
	}
	staged := bundle.Load(files, staging)
	if err := checkStaged(staged, id); err != nil {
		_ = removeTree(files, staging)
		return "", bundle.Bundle{}, err
	}
	return staging, staged, nil
}

// Discard removes a staging folder that will not be published. A missing folder is not an error.
func Discard(files platform.FS, staging string) error {
	return removeTree(files, staging)
}

// Uninstall removes the extension folder id. A missing folder is not an error.
func Uninstall(files platform.FS, clock platform.Clock, root, id string) error {
	target := filepath.Join(root, id)
	if _, err := files.Lstat(target); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	trash := filepath.Join(root, trashPrefix+id+"-"+strconv.FormatInt(clock.Now().UnixNano(), 36))
	if err := files.Rename(target, trash); err != nil {
		return fmt.Errorf("remove extension %s: %w", id, err)
	}
	return removeTree(files, trash)
}

// Sweep deletes staging and trash folders left by an interrupted install or uninstall.
func Sweep(files platform.FS, root string) error {
	entries, err := files.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, stagingPrefix) || strings.HasPrefix(name, trashPrefix) {
			errs = append(errs, removeTree(files, filepath.Join(root, name)))
		}
	}
	return errors.Join(errs...)
}

// checkStaged rejects a staged folder that does not load or that names another extension.
func checkStaged(staged bundle.Bundle, id string) error {
	if len(staged.Problems) > 0 {
		return fmt.Errorf("extension %s is not valid: %s", id, strings.Join(staged.Problems, "; "))
	}
	if staged.Manifest.ID != id {
		return fmt.Errorf("manifest id %q does not match the install id %q", staged.Manifest.ID, id)
	}
	return nil
}

// swapIn moves the staged folder to its final name, moving any previous version aside first and rolling
// back when the final rename fails.
func swapIn(files platform.FS, clock platform.Clock, root, id, staging string) (bool, error) {
	target := filepath.Join(root, id)
	trash := filepath.Join(root, trashPrefix+id+"-"+strconv.FormatInt(clock.Now().UnixNano(), 36))
	previousExists := false
	if _, err := files.Lstat(target); err == nil {
		previousExists = true
		if err := files.Rename(target, trash); err != nil {
			_ = removeTree(files, staging)
			return false, fmt.Errorf("replace extension %s: %w", id, err)
		}
	}
	if err := files.Rename(staging, target); err != nil {
		if previousExists {
			_ = files.Rename(trash, target)
		}
		_ = removeTree(files, staging)
		return false, fmt.Errorf("install extension %s: %w", id, err)
	}
	if previousExists {
		return true, removeTree(files, trash)
	}
	return true, nil
}

func writeFolder(files platform.FS, dir string, contents map[string][]byte) error {
	if err := files.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	for name, data := range contents {
		if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			return fmt.Errorf("file name %q must be a plain name", name)
		}
		if err := files.WriteFile(filepath.Join(dir, name), data, fileMode); err != nil {
			return err
		}
		if err := files.Sync(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// removeTree deletes a folder and everything in it, deepest entries first.
func removeTree(files platform.FS, dir string) error {
	var paths []string
	err := files.Walk(dir, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	for i := len(paths) - 1; i >= 0; i-- {
		errs = append(errs, files.Remove(paths[i]))
	}
	return errors.Join(errs...)
}
