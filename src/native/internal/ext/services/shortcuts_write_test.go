package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingEditor is a shortcut editor that records the changes it is asked for.
type recordingEditor struct {
	set []protocol.ShortcutArgs
}

func (e *recordingEditor) Bindable() []api.ShortcutRecord {
	return []api.ShortcutRecord{{ActionID: "command:notes", Title: "Search Notes"}}
}

func (e *recordingEditor) Plan(actionID, accelerator string) (protocol.ShortcutChange, error) {
	return protocol.ShortcutChange{ActionID: actionID, Accelerator: accelerator}, nil
}

func (e *recordingEditor) Set(_ context.Context, actionID, accelerator, expect string) (api.ShortcutRecord, error) {
	e.set = append(e.set, protocol.ShortcutArgs{ActionID: actionID, Accelerator: accelerator, Expect: expect})
	return api.ShortcutRecord{ActionID: actionID, Accelerator: accelerator}, nil
}

func (e *recordingEditor) Remove(context.Context, string, string) error { return nil }

func serveShortcutWrite(h *Handler, method string, args protocol.ShortcutArgs) protocol.ServiceReply {
	raw, _ := json.Marshal(args)
	ctx := protocol.WithRequestMethod(context.Background(), method)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceShortcuts, Op: protocol.OpSet, Args: raw})
}

func TestShortcutWritesNeedTheWriteCapabilityAndAPaletteRun(t *testing.T) {
	editor := &recordingEditor{}
	args := protocol.ShortcutArgs{ActionID: "command:notes", Accelerator: "Command+Alt+N"}
	readOnly := New(Deps{ShortcutEditor: editor}, manifestFor("acme.reader", api.CapabilityShortcuts))
	requireCode(t, serveShortcutWrite(readOnly, protocol.MethodRun, args), api.CodePermission, "shortcuts.write")
	writer := New(Deps{ShortcutEditor: editor}, manifestFor("acme.writer", api.CapabilityShortcuts, api.CapabilityShortcutsWrite))
	requireCode(t, serveShortcutWrite(writer, protocol.MethodSearch, args), api.CodePermission, "palette action")
	if len(editor.set) != 0 {
		t.Fatalf("a refused write reached the editor: %+v", editor.set)
	}
	mustOK(t, serveShortcutWrite(writer, protocol.MethodRun, args))
	mustOK(t, serveShortcutWrite(writer, protocol.MethodProgram, args))
	if len(editor.set) != 2 {
		t.Fatalf("set = %+v", editor.set)
	}
}

// Listing the bindable commands is a read: the shortcuts capability is enough, from any request.
func TestBindableNeedsOnlyTheShortcutsCapability(t *testing.T) {
	h := New(Deps{ShortcutEditor: &recordingEditor{}}, manifestFor("acme.reader", api.CapabilityShortcuts))
	var records []api.ShortcutRecord
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceShortcuts, protocol.OpBindable, nil)), &records)
	if len(records) != 1 || records[0].Title != "Search Notes" {
		t.Fatalf("records = %+v", records)
	}
}
