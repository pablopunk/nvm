package services

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestOCRContractProbesAndExecutionShareActualPorts(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)
	for _, service := range []*ocr.Service{nil, ocr.New(ocr.Deps{})} {
		deps.OCR = service
		host := desktopHandler(t, deps, api.CapabilityOCR)
		if host.publicCapability("ocr") || host.publicCapability("ocr.screen") {
			t.Fatal("unwired OCR advertised")
		}
		requireCode(t, serve(t, host, protocol.ServiceOCR, protocol.OpScreen, protocol.OCRScreenArgs{}), api.CodeUnavailable, "not available")
	}
}

func TestOCRContractAdmissionRefusesStoppedCallsWithoutCapture(t *testing.T) {
	for _, mode := range []string{"cancelled", "lifetime", "closed"} {
		t.Run(mode, func(t *testing.T) {
			_, deps := newDesktopHost(t, platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)
			text := fake.NewTextRecognizer()
			capture := fake.NewScreenCapturer()
			deps.OCR = ocr.New(ocr.Deps{Text: text, Capture: capture, FS: fake.NewFS(), TempDir: t.TempDir()})
			host := desktopHandler(t, deps, api.CapabilityOCR)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "cancelled":
				cancel()
			case "lifetime":
				lifetime, stop := context.WithCancel(context.Background())
				stop()
				ctx = protocol.WithLifetime(ctx, lifetime)
			case "closed":
				_ = host.Close()
			}
			_, err := host.serveOCR(ctx, protocol.ServiceCall{Op: protocol.OpScreen})
			if err == nil {
				t.Fatal("stopped operation admitted")
			}
			if mode != "closed" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if len(text.Calls()) != 0 || len(capture.Captures()) != 0 {
				t.Fatal("stopped call reached native OCR")
			}
		})
	}
}

func TestOCRContractKeepsPermissionDenialDistinctFromUnsupported(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)
	capture := fake.NewScreenCapturer()
	capture.FailWith(platform.ErrDenied)
	deps.OCR = ocr.New(ocr.Deps{Text: fake.NewTextRecognizer(), Capture: capture, FS: fake.NewFS(), TempDir: t.TempDir()})
	host := desktopHandler(t, deps, api.CapabilityOCR)
	if !host.publicCapability("ocr.screen") {
		t.Fatal("permission readiness changed structural discovery")
	}
	requireCode(t, serve(t, host, protocol.ServiceOCR, protocol.OpScreen, protocol.OCRScreenArgs{}), api.CodePermission, "screen recording")
	if len(capture.Captures()) != 1 {
		t.Fatal("permission refusal boundary not reached")
	}
}
