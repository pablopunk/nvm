package services

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxReadTextBytes bounds what readText returns. A longer file is refused rather than cut, so an extension never works
// on a silently partial text. The reply travels as one protocol line, so the bound stays well under that line's cap.
const maxReadTextBytes = 256 << 10

// readText returns the whole text of an absolute file, refused above maxReadTextBytes.
func (h *Handler) readText(raw json.RawMessage) (string, error) {
	path, err := absolutePathArg(raw)
	if err != nil {
		return "", err
	}
	if h.deps.Files == nil {
		return "", unavailable("files are not available in this build")
	}
	data, size, err := h.deps.Files.ReadFilePrefix(path, maxReadTextBytes+1)
	if err != nil {
		return "", notFound("cannot read %s", files.DisplayPath(h.deps.Home, path))
	}
	if size > maxReadTextBytes {
		return "", invalid("%s is larger than %d KiB and cannot be read as text", filepath.Base(path), maxReadTextBytes>>10)
	}
	return string(data), nil
}

// quickLookFile shows a file in the system's previewer, outside Nevermind. Systems without one refuse it.
func (h *Handler) quickLookFile(raw json.RawMessage) error {
	path, err := absolutePathArg(raw)
	if err != nil {
		return err
	}
	if h.deps.Shell == nil {
		return unavailable("files are not available in this build")
	}
	if !h.actions.allow() {
		return rateExceeded("file opens")
	}
	return unsupportedAsUnavailable(h.deps.Shell.QuickLook(path), "Quick Look is not available on this system")
}

// openWithApp opens a file with one application from the open-with list.
func (h *Handler) openWithApp(raw json.RawMessage) error {
	var args protocol.OpenWithArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	if !filepath.IsAbs(args.Path) || !filepath.IsAbs(args.AppPath) {
		return invalid("a file and an application must be named by absolute paths")
	}
	if h.deps.Shell == nil {
		return unavailable("files are not available in this build")
	}
	if !h.actions.allow() {
		return rateExceeded("file opens")
	}
	app := platform.App{Path: filepath.Clean(args.AppPath), Name: filepath.Base(args.AppPath)}
	return unsupportedAsUnavailable(h.deps.Shell.OpenWith(filepath.Clean(args.Path), app), "this system cannot open a file with a chosen app")
}

// unsupportedAsUnavailable turns a port's ErrUnsupported into a call failure with the sentence the extension shows.
func unsupportedAsUnavailable(err error, sentence string) error {
	if errors.Is(err, platform.ErrUnsupported) {
		return unavailable("%s", sentence)
	}
	return err
}

// displayPathOf formats a path for display, with the home folder shown as "~". It does not change the path itself.
func (h *Handler) displayPathOf(raw json.RawMessage) (string, error) {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	return files.DisplayPath(h.deps.Home, args.Path), nil
}
