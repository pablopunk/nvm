package services

import (
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingIndicators keeps every call the handler forwards to the host's indicator surface.
type recordingIndicators struct {
	mu    sync.Mutex
	shows []string
	hides []string
}

func (r *recordingIndicators) ShowIndicator(owner string, input api.IndicatorInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shows = append(r.shows, owner+"/"+input.ID+"/"+input.Title)
}

func (r *recordingIndicators) HideIndicator(owner, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hides = append(r.hides, owner+"/"+id)
}

func indicatorHandler(t *testing.T, surface IndicatorSurface) *Handler {
	t.Helper()
	_, deps := newDesktopHost(t)
	deps.Indicators = surface
	return desktopHandler(t, deps)
}

func TestIndicatorsShowAndHideUnderTheExtensionsOwnId(t *testing.T) {
	surface := &recordingIndicators{}
	h := indicatorHandler(t, surface)
	mustOK(t, serve(t, h, protocol.ServiceIndicators, protocol.OpShow, api.IndicatorInput{ID: "copy", Title: "Copying"}))
	mustOK(t, serve(t, h, protocol.ServiceIndicators, protocol.OpUpdate, api.IndicatorInput{ID: "copy", Title: "Half"}))
	mustOK(t, serve(t, h, protocol.ServiceIndicators, protocol.OpHide, protocol.IndicatorHideArgs{ID: "copy"}))
	if strings.Join(surface.shows, ",") != "acme.desktop/copy/Copying,acme.desktop/copy/Half" {
		t.Fatalf("shows = %v; want both calls under the extension id", surface.shows)
	}
	if strings.Join(surface.hides, ",") != "acme.desktop/copy" {
		t.Fatalf("hides = %v; want the hide under the extension id", surface.hides)
	}
}

func TestIndicatorNeedsATitle(t *testing.T) {
	surface := &recordingIndicators{}
	h := indicatorHandler(t, surface)
	requireCode(t, serve(t, h, protocol.ServiceIndicators, protocol.OpShow, api.IndicatorInput{ID: "blank"}), api.CodeInvalid, "title")
	if len(surface.shows) != 0 {
		t.Fatalf("an indicator without a title reached the surface: %v", surface.shows)
	}
}

func TestIndicatorsAnswerUnavailableWithoutASurface(t *testing.T) {
	h := indicatorHandler(t, nil)
	requireCode(t, serve(t, h, protocol.ServiceIndicators, protocol.OpShow, api.IndicatorInput{Title: "Working"}), api.CodeUnavailable, "not available")
}

func TestIndicatorUpdatesAreRateLimited(t *testing.T) {
	surface := &recordingIndicators{}
	h := indicatorHandler(t, surface)
	for i := 0; i < indicatorUpdatesPerMinute; i++ {
		mustOK(t, serve(t, h, protocol.ServiceIndicators, protocol.OpUpdate, api.IndicatorInput{Title: "Tick"}))
	}
	requireCode(t, serve(t, h, protocol.ServiceIndicators, protocol.OpUpdate, api.IndicatorInput{Title: "Tick"}), api.CodeQuota, "rate limited")
}
