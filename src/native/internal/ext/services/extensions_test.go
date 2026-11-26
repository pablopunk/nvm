package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingExtensions is the host's extensions port: one extension, and the changes it was asked for.
type recordingExtensions struct {
	calls []string
}

func (e *recordingExtensions) List() []api.ExtensionRecord {
	return []api.ExtensionRecord{{ID: "acme.clock", Title: "Clock", Enabled: true}}
}

func (e *recordingExtensions) Disable(id string) error {
	e.calls = append(e.calls, "disable "+id)
	return nil
}

func (e *recordingExtensions) OpenReview(id string) error {
	e.calls = append(e.calls, "review "+id)
	return nil
}

func (e *recordingExtensions) SetMenuBarItem(id string, shown bool) error {
	e.calls = append(e.calls, "menubar "+id)
	return nil
}

func serveExtensionsCall(h *Handler, method, op string, args any) protocol.ServiceReply {
	raw, _ := json.Marshal(args)
	ctx := protocol.WithRequestMethod(context.Background(), method)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceExtensions, Op: op, Args: raw})
}

func TestListingExtensionsNeedsTheExtensionsDeclaration(t *testing.T) {
	editor := &recordingExtensions{}
	bare := New(Deps{Extensions: editor}, manifestFor("acme.bare"))
	requireCode(t, serveExtensionsCall(bare, protocol.MethodRun, protocol.OpList, nil), api.CodePermission, "extensions")
	reader := New(Deps{Extensions: editor}, manifestFor("acme.reader", api.CapabilityExtensions))
	var records []api.ExtensionRecord
	mustDecode(t, mustOK(t, serveExtensionsCall(reader, protocol.MethodSearch, protocol.OpList, nil)), &records)
	if len(records) != 1 || records[0].ID != "acme.clock" {
		t.Fatalf("records = %+v", records)
	}
}

func TestExtensionChangesNeedTheWriteCapabilityAndAPaletteRun(t *testing.T) {
	editor := &recordingExtensions{}
	clock := protocol.ExtensionArgs{ID: "acme.clock"}
	reader := New(Deps{Extensions: editor}, manifestFor("acme.reader", api.CapabilityExtensions))
	requireCode(t, serveExtensionsCall(reader, protocol.MethodRun, protocol.OpDisable, clock), api.CodePermission, "extensions.write")
	writer := New(Deps{Extensions: editor}, manifestFor("acme.writer", api.CapabilityExtensionsWrite))
	requireCode(t, serveExtensionsCall(writer, protocol.MethodSearch, protocol.OpOpenReview, clock), api.CodePermission, "palette action")
	if len(editor.calls) != 0 {
		t.Fatalf("a refused change reached the host: %v", editor.calls)
	}
	mustOK(t, serveExtensionsCall(writer, protocol.MethodRun, protocol.OpDisable, clock))
	mustOK(t, serveExtensionsCall(writer, protocol.MethodProgram, protocol.OpOpenReview, clock))
	mustOK(t, serveExtensionsCall(writer, protocol.MethodRun, protocol.OpSetMenuBarItem, protocol.ExtensionMenuBarArgs{ID: "acme.clock"}))
	want := []string{"disable acme.clock", "review acme.clock", "menubar acme.clock"}
	if len(editor.calls) != len(want) {
		t.Fatalf("calls = %v; want %v", editor.calls, want)
	}
	for i := range want {
		if editor.calls[i] != want[i] {
			t.Fatalf("calls = %v; want %v", editor.calls, want)
		}
	}
}

// An extension cannot open its own review or turn itself off, so it can never take part in its own enabling.
func TestAnExtensionCannotChangeItsOwnState(t *testing.T) {
	editor := &recordingExtensions{}
	writer := New(Deps{Extensions: editor}, manifestFor("acme.clock", api.CapabilityExtensionsWrite))
	self := protocol.ExtensionArgs{ID: "acme.clock"}
	requireCode(t, serveExtensionsCall(writer, protocol.MethodRun, protocol.OpOpenReview, self), api.CodeInvalid, "its own state")
	requireCode(t, serveExtensionsCall(writer, protocol.MethodRun, protocol.OpDisable, self), api.CodeInvalid, "its own state")
	requireCode(t, serveExtensionsCall(writer, protocol.MethodRun, protocol.OpDisable, protocol.ExtensionArgs{}), api.CodeInvalid, "extension id")
	requireCode(t, serveExtensionsCall(writer, protocol.MethodRun, "enable", self), api.CodeInvalid, "no operation")
	if len(editor.calls) != 0 {
		t.Fatalf("a refused change reached the host: %v", editor.calls)
	}
}

func TestExtensionsAreUnavailableWithoutAPort(t *testing.T) {
	h := New(Deps{}, manifestFor("acme.reader", api.CapabilityExtensions))
	requireCode(t, serveExtensionsCall(h, protocol.MethodRun, protocol.OpList, nil), api.CodeUnavailable, "extensions")
}
