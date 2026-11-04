package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// hideRecorder is the part of the palette host an action touches: it records that the palette hid.
type hideRecorder struct {
	search.Host
	hidden bool
}

func (h *hideRecorder) Hide() { h.hidden = true }

func TestOpenSystemSettingsActionHidesThePaletteAndOpensThePane(t *testing.T) {
	var opened []string
	deps := Deps{OpenSettingsPane: func(id string) { opened = append(opened, id) }}
	action := ActionFor(deps, "nevermind.system-settings", api.Action{Kind: api.ActionSystem, Op: "open-system-settings", Setting: "com.apple.Sound-Settings.extension", Title: "Open"})
	host := &hideRecorder{}
	action.Run(host)
	if !host.hidden || len(opened) != 1 || opened[0] != "com.apple.Sound-Settings.extension" {
		t.Fatalf("hidden %v, opened %v", host.hidden, opened)
	}
}

func TestSystemActionWithoutAHostOperationIsUnavailable(t *testing.T) {
	var toasts []string
	action := ActionFor(Deps{}, "nevermind.system-settings", api.Action{Kind: api.ActionSystem, Op: "open-system-settings", Setting: "x"})
	host := &toastRecorder{toasts: &toasts}
	action.Run(host)
	if len(toasts) != 1 || toasts[0] != unavailableMessage {
		t.Fatalf("toasts = %v, want the unavailable message", toasts)
	}
}

type toastRecorder struct {
	search.Host
	toasts *[]string
}

func (h *toastRecorder) Toast(msg string) { *h.toasts = append(*h.toasts, msg) }
