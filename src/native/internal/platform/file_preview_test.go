package platform_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPreviewSupportAndExecutionShareTheBinding(t *testing.T) {
	for _, supported := range []bool{false, true} {
		shell := fake.NewShell()
		var caps platform.Capabilities
		if supported {
			caps = platform.NewCapabilities(platform.CapabilityShellQuickLook)
		}
		preview := platform.BindFilePreview(shell, caps)
		err := preview.Show(context.Background(), "/tmp/a.pdf")
		if preview.Supported() != supported || (!supported && !errors.Is(err, platform.ErrUnsupported)) {
			t.Fatalf("supported=%t error=%v", preview.Supported(), err)
		}
		want := 0
		if supported {
			want = 1
		}
		if len(shell.Calls()) != want {
			t.Fatalf("effects=%v", shell.Calls())
		}
	}
	if platform.BindFilePreview(nil, platform.NewCapabilities(platform.CapabilityShellQuickLook)).Supported() {
		t.Fatal("missing executor advertised support")
	}
}

func TestCancelledPreviewNeverReachesTheOS(t *testing.T) {
	shell := fake.NewShell()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := platform.BindFilePreview(shell, platform.NewCapabilities(platform.CapabilityShellQuickLook)).Show(ctx, "/tmp/a.pdf")
	if !errors.Is(err, context.Canceled) || len(shell.Calls()) != 0 {
		t.Fatalf("error=%v effects=%v", err, shell.Calls())
	}
}
