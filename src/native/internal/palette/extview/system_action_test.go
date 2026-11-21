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
	identity := new(byte)
	deps := Deps{CommandSnapshotIdentity: func(string) any { return identity }, SystemCommand: func(id, op, pane string, got any, done func(string)) {
		if id != "nevermind.system-settings" || op != "open-system-settings" || got != identity {
			t.Fatal("pane escaped originating system route")
		}
		opened = append(opened, pane)
		done("")
	}}
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

func TestSystemSettingsPaneConfirmationPreservesTargetAndInstallation(t *testing.T) {
	identity := new(byte)
	calls := 0
	deps := Deps{CommandSnapshotIdentity: func(string) any { return identity }, SystemCommand: func(id, op, pane string, actor any, done func(string)) {
		calls++
		if id != "acme.settings" || op != "open-system-settings" || pane != "original-pane" || actor != identity {
			t.Fatal("confirmation changed originating settings request")
		}
		done("")
	}}
	session := NewSession(deps, "acme.settings", api.ListView("Settings"))
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionSystem, Op: "open-system-settings", Setting: "original-pane", Confirm: &api.Confirmation{}}
	session.action(action, "settings").Run(host)
	if calls != 0 || len(host.pushed) != 1 || host.hidden {
		t.Fatal("settings pane bypassed confirmation")
	}
	action.Setting = "changed-pane"
	confirm := host.pushed[0].Search("")[0].Actions[0]
	confirm.Run(host)
	confirm.Run(host)
	if calls != 1 || !host.hidden {
		t.Fatal("settings confirmation was not a single-use host route")
	}
}
