package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestPreviewRoutesPreserveAccessPolicyAndShareNativeEffect(t *testing.T) {
	for _, declared := range []bool{false, true} {
		host, deps := newDesktopHost(t, platform.CapabilityShellQuickLook)
		var caps []api.Capability
		if declared {
			caps = append(caps, api.CapabilityDesktopFiles)
		}
		handler := desktopHandler(t, deps, caps...)
		path := filepath.Join(t.TempDir(), "a.pdf")
		reply := serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpQuickLook, protocol.PathArgs{Path: path})
		if declared {
			mustOK(t, reply)
		} else {
			requireCode(t, reply, api.CodePermission, "desktop.files")
		}
		if err := handler.PreviewFileAction(context.Background(), path); err != nil {
			t.Fatal(err)
		}
		want := 1
		if declared {
			want++
		}
		if len(host.shell.Calls()) != want {
			t.Fatalf("declared=%t effects=%v", declared, host.shell.Calls())
		}
		if err := handler.Close(); err != nil {
			t.Fatal(err)
		}
		if err := handler.PreviewFileAction(context.Background(), path); err == nil {
			t.Fatal("closed actor performed preview")
		}
		if len(host.shell.Calls()) != want {
			t.Fatal("revoked actor reached OS")
		}
	}
}

func TestPreviewRejectsUnsupportedCancelledAndRelativeCallsWithoutEffects(t *testing.T) {
	host, deps := newDesktopHost(t)
	handler := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	path := filepath.Join(t.TempDir(), "a.pdf")
	requireCode(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpQuickLook, protocol.PathArgs{Path: path}), api.CodeUnavailable, "")
	if err := handler.PreviewFileAction(context.Background(), path); err == nil {
		t.Fatal("unsupported preview succeeded")
	}
	if err := handler.PreviewFileAction(context.Background(), "relative.pdf"); err == nil {
		t.Fatal("relative path reached native preview")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := handler.PreviewFileAction(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(host.shell.Calls()) != 0 {
		t.Fatal("refused call reached OS")
	}
}
