package manager

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// goodSource is a source that compiles against the api symbols and implements a root row.
const goodSource = `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func RootItems(ctx api.Context) ([]api.Item, error) { return nil, nil }
`

// TestCheckAcceptsAGoodSourceWithoutPublishingIt: the candidate loads and reports its entry points, and the folder
// never appears under the extensions root.
func TestCheckAcceptsAGoodSourceWithoutPublishingIt(t *testing.T) {
	h := newHarness(t)
	methods, err := h.m.Check("acme.good", fixtureContents(t, "acme.good", goodSource))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Contains(methods, "root") {
		t.Fatalf("methods = %v; want the root entry point", methods)
	}
	if got := h.m.List(); len(got) != 0 {
		t.Fatalf("a check published %d extension(s)", len(got))
	}
	if entries, _ := h.files.ReadDir(h.root); len(entries) != 0 {
		t.Fatalf("a check left %d entries in the extensions folder", len(entries))
	}
}

// TestCheckReportsTheCompileErrorAndLeavesNothing: an unknown api name is a compile error the model can read.
func TestCheckReportsTheCompileErrorAndLeavesNothing(t *testing.T) {
	h := newHarness(t)
	broken := strings.Replace(goodSource, "api.Item", "api.Itme", 1)
	_, err := h.m.Check("acme.good", fixtureContents(t, "acme.good", broken))
	if err == nil || !strings.Contains(err.Error(), "does not compile") {
		t.Fatalf("Check error = %v; want the compile error", err)
	}
	if entries, _ := h.files.ReadDir(h.root); len(entries) != 0 {
		t.Fatalf("a failed check left %d entries behind", len(entries))
	}
}

// TestCheckRefusesAnEntryPointWithTheWrongSignature: a wrong parameter count is refused at load, named by function.
func TestCheckRefusesAnEntryPointWithTheWrongSignature(t *testing.T) {
	h := newHarness(t)
	wrong := strings.Replace(goodSource, "RootItems(ctx api.Context) ([]api.Item, error)", "RootItems() ([]api.Item, error)", 1)
	_, err := h.m.Check("acme.good", fixtureContents(t, "acme.good", wrong))
	if err == nil || !strings.Contains(err.Error(), "RootItems has the wrong signature") {
		t.Fatalf("Check error = %v; want the signature refusal", err)
	}
}

// TestCheckRefusesAnEntryPointWithTheWrongType: the count is right but a result type is not the api's, so the
// entry point is refused at load rather than at its first call.
func TestCheckRefusesAnEntryPointWithTheWrongType(t *testing.T) {
	h := newHarness(t)
	wrong := strings.Replace(goodSource, "RootItems(ctx api.Context) ([]api.Item, error)", "RootItems(ctx api.Context) ([]string, error)", 1)
	_, err := h.m.Check("acme.good", fixtureContents(t, "acme.good", wrong))
	if err == nil || !strings.Contains(err.Error(), "RootItems has the wrong signature") {
		t.Fatalf("Check error = %v; want the signature refusal for the result type", err)
	}
}

// TestInstallWithoutEnableLeavesTheExtensionOff: a reinstall of an enabled extension is off until it is reviewed.
func TestInstallWithoutEnableLeavesTheExtensionOff(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good", goodSource), false); err != nil {
		t.Fatal(err)
	}
	listing := h.listingFor("acme.good")
	if listing.Enabled {
		t.Fatal("a new version installed with enable false is still enabled")
	}
	definition, source, err := h.m.Definition("acme.good")
	if err != nil || definition.ID != "acme.good" || !strings.Contains(source, "RootItems") {
		t.Fatalf("Definition = %+v, %v; want the installed version", definition, err)
	}
	if got := filepath.Base(listing.Dir); got != "acme.good" {
		t.Fatalf("folder = %q", got)
	}
}

func (h *harness) listingFor(id string) Listing {
	h.t.Helper()
	for _, listing := range h.m.List() {
		if listing.ID == id {
			return listing
		}
	}
	h.t.Fatalf("no listing for %s", id)
	return Listing{}
}

// TestDisabledInstallContributesNoCommandsOrSearchRows: an extension installed off has no root commands and no search
// rows, which is what the palette's root and search show.
func TestDisabledInstallContributesNoCommandsOrSearchRows(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), false); err != nil {
		t.Fatal(err)
	}
	for _, result := range h.m.Commands(context.Background()) {
		if result.ExtensionID == "acme.good" {
			t.Fatalf("a disabled extension declared commands: %+v", result)
		}
	}
	if result := resultFor(h.search("alpha"), "acme.good"); result != nil {
		t.Fatalf("a disabled extension answered search: %+v", result)
	}
}

// TestCheckRefusesAScriptWhoseCapabilityIsUndeclared is the owner's refusal before it reaches a child: a theme toggle
// that runs AppleScript without declaring system is refused with the capability it needs, and the same source loads
// once system is declared.
func TestCheckRefusesAScriptWhoseCapabilityIsUndeclared(t *testing.T) {
	h := newHarness(t)
	undeclared := fixtureContents(t, "acme.theme")
	undeclared[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme"}`)
	_, err := h.m.Check("user.system-theme", undeclared)
	if err == nil || !strings.Contains(err.Error(), `"system" capability`) {
		t.Fatalf("Check error = %v; want the system capability named", err)
	}
	methods, err := h.m.Check("acme.theme", fixtureContents(t, "acme.theme"))
	if err != nil {
		t.Fatalf("the declared version failed the check: %v", err)
	}
	if !slices.Contains(methods, "commands") || !slices.Contains(methods, "run") {
		t.Fatalf("methods = %v; want the commands and run entry points", methods)
	}
}
