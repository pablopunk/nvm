package services

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// maxWriteBytes is the largest binary file one call writes. The data travels base64-encoded in one protocol line,
// which the host caps at protocol.MaxLineBytes, so the cap leaves room for the encoding and the envelope.
const maxWriteBytes = 512 << 10

// writeBytes saves raw bytes as a file in an existing folder. It fails when the file exists unless overwrite is set,
// and it replaces the file atomically: the bytes go to a temporary file in the same folder, then rename.
func (h *Handler) writeBytes(raw json.RawMessage) (api.FileRef, error) {
	var args protocol.WriteBytesArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.FileRef{}, err
	}
	return h.writeContent(args.Path, args.Data, args.Overwrite)
}

// writeText saves UTF-8 text the same way writeBytes saves bytes.
func (h *Handler) writeText(raw json.RawMessage) (api.FileRef, error) {
	var args protocol.WriteFileTextArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.FileRef{}, err
	}
	return h.writeContent(args.Path, []byte(args.Text), args.Overwrite)
}

// writeContent is the shared write of writeBytes and writeText: the path must be absolute and its folder must exist.
func (h *Handler) writeContent(rawPath string, data []byte, overwrite bool) (api.FileRef, error) {
	if !filepath.IsAbs(rawPath) {
		return api.FileRef{}, invalid("a file path must be absolute")
	}
	if len(data) > maxWriteBytes {
		return api.FileRef{}, invalid("a file write is limited to %d bytes per call", maxWriteBytes)
	}
	if h.deps.Files == nil {
		return api.FileRef{}, unavailable("files are not available in this build")
	}
	path := filepath.Clean(rawPath)
	if err := h.checkWritableTarget(path, overwrite); err != nil {
		return api.FileRef{}, err
	}
	if !h.actions.allow() {
		return api.FileRef{}, rateExceeded("file writes")
	}
	if err := h.replaceFile(path, data); err != nil {
		return api.FileRef{}, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	info, err := h.deps.Files.Stat(path)
	if err != nil {
		return api.FileRef{}, fmt.Errorf("read back %s: %w", filepath.Base(path), err)
	}
	return fileRefOf(path, h.deps.Home, info.Size(), info.ModTime(), time.Time{}), nil
}

// checkWritableTarget refuses a write whose folder does not exist, or whose file exists without overwrite. It never
// creates folders.
func (h *Handler) checkWritableTarget(path string, overwrite bool) error {
	folder, err := h.deps.Files.Stat(filepath.Dir(path))
	if err != nil || !folder.IsDir() {
		return notFound("the folder of %s does not exist", filepath.Base(path))
	}
	if _, err := h.deps.Files.Lstat(path); err == nil && !overwrite {
		return serviceError{
			failure: api.Failure{Code: api.CodeInvalid, Message: fmt.Sprintf("%s already exists; pass overwrite to replace it", filepath.Base(path))},
			cause:   api.ErrInvalidInput,
		}
	}
	return nil
}

// replaceFile writes data to a temporary file beside path, flushes it and renames it over path.
func (h *Handler) replaceFile(path string, data []byte) error {
	temp := fmt.Sprintf("%s.%d.part", path, h.deps.Clock.Now().UnixNano())
	if err := h.deps.Files.WriteFile(temp, data, fs.FileMode(0o644)); err != nil {
		return err
	}
	if err := h.deps.Files.Sync(temp); err != nil {
		_ = h.deps.Files.Remove(temp)
		return err
	}
	if err := h.deps.Files.Rename(temp, path); err != nil {
		_ = h.deps.Files.Remove(temp)
		return err
	}
	return nil
}
