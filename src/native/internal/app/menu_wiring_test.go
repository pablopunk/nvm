package app

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/menushost"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type menuCapture struct {
	calls     chan int
	cancelled chan struct{}
}

func (m *menuCapture) Capture(ctx context.Context, pid int) (platform.MenuSource, error) {
	m.calls <- pid
	<-ctx.Done()
	m.cancelled <- struct{}{}
	return platform.MenuSource{}, ctx.Err()
}
func (*menuCapture) Snapshot(context.Context, platform.MenuSource) ([]platform.MenuEntry, error) {
	return nil, nil
}
func (*menuCapture) Invoke(context.Context, platform.MenuSource, string) error { return nil }

func TestMenuCaptureUsesOriginalFrontmostAndHideCancels(t *testing.T) {
	p := goldenPalette(t)
	p.ports.Capabilities = platform.NewCapabilities()
	windows := p.ports.Windows.(*fake.WindowManager)
	original := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "original"})
	windows.Focus(original)
	source := &menuCapture{make(chan int, 1), make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.menus = menushost.New(ctx, source, time.Minute, nil)
	p.captureMenusOnShow()
	other := windows.AddWindow(fake.SimWindow{PID: 77, BundleID: "nevermind"})
	windows.Focus(other)
	if pid := <-source.calls; pid != 42 {
		t.Fatalf("captured PID %d", pid)
	}
	if p.menuPort() != p.menus {
		t.Fatal("menu port not forwarded")
	}
	p.onPaletteHide()
	<-source.cancelled
	if _, err := p.menuPort().Snapshot(context.Background()); err == nil {
		t.Fatal("dismissed menu available")
	}
	p.menus.Close()
}

func TestMenuPortNilWhenUnsupported(t *testing.T) {
	p := &App{}
	if p.menuPort() != nil {
		t.Fatal("unsupported menu port is available")
	}
}
