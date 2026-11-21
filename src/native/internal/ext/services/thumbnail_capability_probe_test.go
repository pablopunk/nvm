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

func TestThumbnailDoesNotReadTheFile(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := realFilesHandler(t, deps)
	root := t.TempDir()
	bad := filepath.Join(root, "corrupt.png")
	if err := os.WriteFile(bad, []byte("not a png"), 0600); err != nil {
		t.Fatal(err)
	}
	thumbnail := func(path string) protocol.ThumbnailResult {
		var result protocol.ThumbnailResult
		if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path})), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := thumbnail(bad); !got.Found || got.URL == "" {
		t.Fatalf("a file that is not read was refused: %+v", got)
	}
	if got := thumbnail(filepath.Join(root, "missing.png")); got.Found || got.URL != "" {
		t.Fatalf("a missing file has a thumbnail: %+v", got)
	}
}
