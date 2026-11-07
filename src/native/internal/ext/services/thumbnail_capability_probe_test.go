package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestThumbnailClipboardCapabilityIsolationProbe(t *testing.T) {
	_, deps := newDesktopHost(t)
	files := realFilesHandler(t, deps)
	path := writePNG(t, filepath.Join(t.TempDir(), "image.png"), 32, 32)
	var thumbnail protocol.ThumbnailResult
	if err := json.Unmarshal(mustOK(t, serve(t, files, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path})), &thumbnail); err != nil || !thumbnail.Found {
		t.Fatalf("thumbnail: %+v %v", thumbnail, err)
	}
	requireCode(t, serve(t, files, protocol.ServiceDesktopClipboard, protocol.OpWriteImage, protocol.WriteImageArgs{DataURL: thumbnail.URL}), api.CodePermission, "desktop.clipboard")
	clipboard := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, clipboard, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path}), api.CodePermission, "desktop.files")
}

func TestThumbnailMissingAndCorruptRemainAbsentProbe(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := realFilesHandler(t, deps)
	root := t.TempDir()
	bad := filepath.Join(root, "corrupt.png")
	if err := os.WriteFile(bad, []byte("not a png"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{bad, filepath.Join(root, "missing.png")} {
		var result protocol.ThumbnailResult
		err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path})), &result)
		if err != nil || result.Found || result.URL != "" {
			t.Fatalf("%s: %+v, %v", path, result, err)
		}
	}
}

// A path, size and timestamp cannot establish whether the PNG-returning contract will succeed.
func TestThumbnailMetadataCannotDistinguishCorruptionProbe(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := realFilesHandler(t, deps)
	path := writePNG(t, filepath.Join(t.TempDir(), "same.png"), 32, 32)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	read := func() protocol.ThumbnailResult {
		var result protocol.ThumbnailResult
		err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path})), &result)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := read(); !got.Found {
		t.Fatal("valid PNG absent")
	}
	data[0] ^= 0xff
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("metadata changed: %v %+v", err, after)
	}
	if got := read(); got.Found {
		t.Fatalf("corrupt PNG accepted: %+v", got)
	}
}
