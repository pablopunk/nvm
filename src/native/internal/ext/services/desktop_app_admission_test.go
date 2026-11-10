package services

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

type admissionApps struct {
	*lifecycleRecorder
	onIndex func()
	quitAll int
}

func (a *admissionApps) IndexedApps() []appfeature.App {
	if a.onIndex != nil {
		a.onIndex()
	}
	return a.lifecycleRecorder.IndexedApps()
}

func (a *admissionApps) QuitAllApps() ([]appfeature.App, error) {
	a.quitAll++
	return a.lifecycleRecorder.QuitAllApps()
}

func TestAppAdmissionStopsCancelledAndClosedCalls(t *testing.T) {
	for _, closed := range []bool{false, true} {
		for _, op := range []string{protocol.OpLaunch, protocol.OpQuit, protocol.OpForceQuit, protocol.OpQuitAll, protocol.OpHideApp, protocol.OpRunning} {
			t.Run(op+map[bool]string{false: "-cancelled", true: "-closed"}[closed], func(t *testing.T) {
				port := &admissionApps{lifecycleRecorder: &lifecycleRecorder{}}
				h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppQuit, platform.CapabilityAppHide, platform.CapabilityAppRunning)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if closed {
					_ = h.Close()
				} else {
					cancel()
				}
				raw := mustRaw(t, protocol.PathArgs{Path: notesApp})
				if op == protocol.OpRunning {
					raw = mustRaw(t, protocol.PathsArgs{Paths: []string{notesApp}})
				}
				_, err := h.serveDesktopApps(ctx, protocol.ServiceCall{Op: op, Args: raw})
				expected := error(context.Canceled)
				if closed {
					expected = api.ErrNotAvailable
				}
				if !errors.Is(err, expected) {
					t.Fatalf("admission error: %v", err)
				}
				shell := h.deps.Shell.(*fake.Shell)
				if len(port.quit)+len(port.forced)+len(port.hidden)+len(port.asked)+port.quitAll+len(shell.Calls()) != 0 {
					t.Fatal("refused call reached native effect")
				}
			})
		}
	}
}

func TestAppAdmissionRechecksAfterIndexedLookup(t *testing.T) {
	for _, op := range []string{protocol.OpLaunch, protocol.OpQuit} {
		for _, closeHost := range []bool{false, true} {
			t.Run(op+map[bool]string{false: "-cancel", true: "-close"}[closeHost], func(t *testing.T) {
				port := &admissionApps{lifecycleRecorder: &lifecycleRecorder{}}
				h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppQuit)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				port.onIndex = func() {
					if closeHost {
						_ = h.Close()
					} else {
						cancel()
					}
				}
				_, err := h.serveDesktopApps(ctx, protocol.ServiceCall{Op: op, Args: mustRaw(t, protocol.PathArgs{Path: notesApp})})
				expected := error(context.Canceled)
				if closeHost {
					expected = api.ErrNotAvailable
				}
				if !errors.Is(err, expected) {
					t.Fatal(err)
				}
				if len(port.quit) != 0 || len(h.deps.Shell.(*fake.Shell).Calls()) != 0 {
					t.Fatal("effect admitted after lookup invalidated its caller")
				}
			})
		}
	}
}

func TestAppAvailabilityNeedsExecutorWithoutReadingIndex(t *testing.T) {
	names := []string{"apps.running", "apps.quit", "apps.hide", "apps.uninstall"}
	capabilities := []platform.Capability{platform.CapabilityAppRunning, platform.CapabilityAppQuit, platform.CapabilityAppHide, platform.CapabilityAppUninstall}
	h, _, _ := lifecycleTestHandler(t, nil, capabilities...)
	for _, name := range names {
		if h.publicCapability(name) {
			t.Fatal("missing executor reported support", name)
		}
	}
	h.deps.Apps = &admissionApps{lifecycleRecorder: &lifecycleRecorder{}, onIndex: func() { t.Fatal("support probe read a warming index") }}
	for _, name := range names {
		if !h.publicCapability(name) {
			t.Fatal("empty index became unsupported", name)
		}
	}
	h.deps.Capabilities = platform.NewCapabilities()
	if h.publicCapability("apps.quit") {
		t.Fatal("unsupported OS reported support")
	}
}

func TestAppPermissionAndUnsupportedFailuresKeepMessages(t *testing.T) {
	for _, launch := range []bool{false, true} {
		for _, denied := range []bool{false, true} {
			t.Run(map[bool]string{false: "quit", true: "launch"}[launch]+map[bool]string{false: "-unsupported", true: "-denied"}[denied], func(t *testing.T) {
				nativeErr, code := error(platform.ErrUnsupported), api.CodeUnavailable
				if denied {
					nativeErr, code = platform.ErrDenied, api.CodePermission
				}
				port := &lifecycleRecorder{err: nativeErr}
				h, notices, warned := lifecycleTestHandler(t, port, platform.CapabilityAppQuit)
				op := protocol.OpQuit
				if launch {
					op = protocol.OpLaunch
					h.deps.Shell.(*fake.Shell).FailNext(fake.OpShellLaunchApp, nativeErr)
				}
				reply := serve(t, h, protocol.ServiceDesktopApps, op, protocol.PathArgs{Path: notesApp})
				requireCode(t, reply, code, nativeErr.Error())
				if !launch && (len(*notices) != 1 || !warned("the app action failed")) {
					t.Fatal("lifecycle failure lost its existing HUD/log behavior")
				}
			})
		}
	}
}
