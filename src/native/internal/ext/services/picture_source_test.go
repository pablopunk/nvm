package services

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"io"
	"path/filepath"
	"testing"
)

func TestPictureSourceNeedsDeclaredFilesAndLiveHost(t *testing.T) {
	for _, declared := range []bool{false, true} {
		host, deps := newDesktopHost(t)
		path := filepath.Join(t.TempDir(), "source.png")
		files.Seed(path, []byte("pixels"))
		var caps []api.Capability
		if declared {
			caps = append(caps, api.CapabilityDesktopFiles)
		}
		handler := desktopHandler(t, deps, caps...)
		handler.deps.Files = files
		source, err := handler.OpenPictureSource(context.Background(), path)
		if !declared {
			if source != nil || !errors.Is(err, api.ErrPermission) {
				t.Fatalf("undeclared picture = %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(source)
		_ = source.Close()
		if err != nil || string(data) != "pixels" {
			t.Fatalf("source %q, %v", data, err)
		}
		if err := handler.Close(); err != nil {
			t.Fatal(err)
		}
		if source, err := handler.OpenPictureSource(context.Background(), path); err == nil {
			_ = source.Close()
			t.Fatal("closed host authorized a deferred read")
		}
	}
}
