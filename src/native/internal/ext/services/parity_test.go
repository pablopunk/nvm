package services

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func templateText(t *testing.T, h *Handler, input string, options api.TemplateOptions) string {
	t.Helper()
	raw := mustOK(t, serve(t, h, protocol.ServiceText, protocol.OpTemplate, protocol.TemplateArgs{Input: input, Options: options}))
	var result api.TemplateResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result.Text
}

func TestTemplateFillsSelectedTextWhenAskedAndPermitted(t *testing.T) {
	clock := fake.NewClock()
	permitted := desktopHandler(t, Deps{Clock: clock, Selection: &filesSelection{}}, api.CapabilityDesktopSelection)
	if got := templateText(t, permitted, "<{selectedText}>", api.TemplateOptions{}); got != "<text>" {
		t.Fatalf("got %q", got)
	}
	off := false
	if got := templateText(t, permitted, "<{selectedText}>", api.TemplateOptions{IncludeSelectedText: &off}); got != "<>" {
		t.Fatalf("opted out, got %q", got)
	}
	undeclared := desktopHandler(t, Deps{Clock: clock, Selection: &filesSelection{}})
	if got := templateText(t, undeclared, "<{selectedText}>", api.TemplateOptions{}); got != "<>" {
		t.Fatalf("undeclared, got %q", got)
	}
}

func TestRenameTargetsTheCallingExtension(t *testing.T) {
	var gotID string
	var got api.ExtensionRename
	h := desktopHandler(t, Deps{Rename: func(id string, change api.ExtensionRename) error {
		gotID, got = id, change
		return nil
	}})
	mustOK(t, serve(t, h, protocol.ServiceExtension, protocol.OpRename, api.ExtensionRename{Title: "New", CommandID: "c", CommandTitle: "Run"}))
	if gotID != "acme.desktop" || got.Title != "New" || got.CommandTitle != "Run" {
		t.Fatalf("id %q change %+v", gotID, got)
	}
	failing := desktopHandler(t, Deps{Rename: func(string, api.ExtensionRename) error { return errors.New("no folder") }})
	if reply := serve(t, failing, protocol.ServiceExtension, protocol.OpRename, api.ExtensionRename{Title: "New"}); reply.Failure == nil {
		t.Fatal("a failed rename reported success")
	}
	if reply := serve(t, desktopHandler(t, Deps{}), protocol.ServiceExtension, protocol.OpRename, api.ExtensionRename{Title: "New"}); reply.Failure == nil {
		t.Fatal("a host without rename reported success")
	}
}

func TestWindowCapabilitiesReachExtensions(t *testing.T) {
	h := desktopHandler(t, Deps{Capabilities: platform.NewCapabilities(platform.CapabilityWindowAllSpaces)})
	for name, want := range map[string]bool{"windows.all-spaces": true, "windows.frame-restore": false} {
		var supported bool
		raw := mustOK(t, serve(t, h, protocol.ServiceSystem, protocol.OpHasCapability, protocol.CapabilityArgs{Name: name}))
		if err := json.Unmarshal(raw, &supported); err != nil {
			t.Fatal(err)
		}
		if supported != want {
			t.Errorf("%s = %v, want %v", name, supported, want)
		}
	}
}
