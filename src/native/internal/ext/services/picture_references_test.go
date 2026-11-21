package services

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"path/filepath"
	"testing"
)

func TestGridPictureRequiresAnExactReferenceIssuedByItsHost(t *testing.T) {
	_, deps := newDesktopHost(t)
	owner := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	files := owner.deps.Files.(*fake.FS)
	path := filepath.Join(t.TempDir(), "image.png")
	files.Seed(path, []byte("pixels"))
	var picture protocol.ThumbnailResult
	decodeReply(t, serve(t, owner, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path}), &picture)
	if !picture.Found {
		t.Fatal("host did not issue a picture")
	}
	got, source, err := owner.OpenGridPictureSource(context.Background(), picture.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = source.Close()
	if got != filepath.Clean(path) {
		t.Fatal("reference changed its canonical path")
	}
	if _, source, err := owner.OpenGridPictureSource(context.Background(), picture.URL+"forged"); err == nil {
		_ = source.Close()
		t.Fatal("forged picture was opened")
	}
	other := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	other.deps.Files = files
	if _, source, err := other.OpenGridPictureSource(context.Background(), picture.URL); err == nil {
		_ = source.Close()
		t.Fatal("another host inherited picture provenance")
	}
}
