package supervisor

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pasteRig is the services rig with the selection engine over fake ports, so typing and pasting land in the fake paster.
func pasteRig(t *testing.T) (servicesRig, *fake.Paster) {
	t.Helper()
	rig := newServicesRig(t)
	set := fake.NewSet()
	source := platform.FocusTarget{Window: "0x1200007", PID: 4242}
	rig.deps.Paste = selection.New(selection.Deps{
		Clock:        set.Clock,
		Clipboard:    set.Clipboard,
		SelectedText: set.SelectedText,
		Paster:       set.Paster,
		Focus:        set.Focus,
		Source:       func() platform.FocusTarget { return source },
	})
	return rig, set.Paster
}

// TestPasteFixtureTypesOnlyWhenTheKeyboardIsDeclared runs the fixture's typing probe in its child. The keyboard service
// types through the paster when desktop.keyboard is declared, and refuses without it.
func TestPasteFixtureTypesOnlyWhenTheKeyboardIsDeclared(t *testing.T) {
	rig, paster := pasteRig(t)
	declared := servicedRuntime(t, "acme.paste", rig, fixtureManifest("acme.paste", api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard))
	if got := rowTitle(t, declared, "type"); got != "type sent=true" {
		t.Fatalf("typing row = %q, want the keyboard to type", got)
	}
	if typed := paster.Typed(); len(typed) != 1 || typed[0] != "nvm paste fixture" {
		t.Fatalf("typed = %v, want the fixture text", typed)
	}

	undeclared := servicedRuntime(t, "acme.paste", rig, fixtureManifest("acme.paste"))
	if got := rowTitle(t, undeclared, "type"); got != "type sent=false" {
		t.Fatalf("undeclared typing row = %q, want the keyboard refused", got)
	}
	if typed := paster.Typed(); len(typed) != 1 {
		t.Fatalf("refused typing reached the paster: %v", typed)
	}
}

// TestPasteFixturePasteActionCrossesTheChildAndPastes: the paste action the child returns arrives as a declarative
// paste, and the host's services perform it through the paster.
func TestPasteFixturePasteActionCrossesTheChildAndPastes(t *testing.T) {
	rig, paster := pasteRig(t)
	manifest := fixtureManifest("acme.paste", api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	r := servicedRuntime(t, "acme.paste", rig, manifest)

	response, err := callSearch(t, r, "paste")
	if err != nil || response.Failure != nil {
		t.Fatalf("paste row: %v %+v", err, response.Failure)
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil || len(items) != 1 || items[0].Primary == nil {
		t.Fatalf("paste row returned %s (%v); want one row with a primary action", response.Data, err)
	}
	action := *items[0].Primary
	if action.Kind != api.ActionPaste || action.Content == nil || action.Content.Text != "nvm paste fixture" {
		t.Fatalf("primary = %+v; want a paste of the fixture text", action)
	}

	host := services.New(rig.deps, manifest)
	t.Cleanup(func() { _ = host.Close() })
	if err := host.PerformPaste(action); err != nil {
		t.Fatalf("perform paste: %v", err)
	}
	if got := paster.Requests(); len(got) != 1 {
		t.Fatalf("paste requests = %v, want the one paste", got)
	}
}
