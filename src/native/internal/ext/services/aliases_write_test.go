package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingAliasEditor is an alias editor that records the changes it is asked for.
type recordingAliasEditor struct {
	added   []protocol.AliasArgs
	removed []string
}

func (e *recordingAliasEditor) PlanAlias(actionID, alias string) (protocol.AliasChange, error) {
	return protocol.AliasChange{ActionID: actionID, Alias: alias}, nil
}

func (e *recordingAliasEditor) AddAlias(actionID, alias string) error {
	e.added = append(e.added, protocol.AliasArgs{ActionID: actionID, Alias: alias})
	return nil
}

func (e *recordingAliasEditor) RemoveAlias(alias string) error {
	e.removed = append(e.removed, alias)
	return nil
}

func serveAliasWrite(h *Handler, method, op string, args protocol.AliasArgs) protocol.ServiceReply {
	raw, _ := json.Marshal(args)
	ctx := protocol.WithRequestMethod(context.Background(), method)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceShortcuts, Op: op, Args: raw})
}

func TestAliasWritesNeedTheWriteCapabilityAndAPaletteRun(t *testing.T) {
	editor := &recordingAliasEditor{}
	add := protocol.AliasArgs{ActionID: "command:notes", Alias: "jot"}
	readOnly := New(Deps{AliasEditor: editor}, manifestFor("acme.reader", api.CapabilityShortcuts))
	requireCode(t, serveAliasWrite(readOnly, protocol.MethodRun, protocol.OpAddAlias, add), api.CodePermission, "shortcuts.write")
	writer := New(Deps{AliasEditor: editor}, manifestFor("acme.writer", api.CapabilityShortcuts, api.CapabilityShortcutsWrite))
	requireCode(t, serveAliasWrite(writer, protocol.MethodSearch, protocol.OpAddAlias, add), api.CodePermission, "palette action")
	if len(editor.added) != 0 {
		t.Fatalf("a refused write reached the editor: %+v", editor.added)
	}
	mustOK(t, serveAliasWrite(writer, protocol.MethodRun, protocol.OpAddAlias, add))
	mustOK(t, serveAliasWrite(writer, protocol.MethodProgram, protocol.OpRemoveAlias, protocol.AliasArgs{Alias: "jot"}))
	if len(editor.added) != 1 || editor.added[0] != add || len(editor.removed) != 1 || editor.removed[0] != "jot" {
		t.Fatalf("added = %+v, removed = %+v", editor.added, editor.removed)
	}
}

func TestAddingAnAliasNeedsTheActionAndTheAlias(t *testing.T) {
	writer := New(Deps{AliasEditor: &recordingAliasEditor{}}, manifestFor("acme.writer", api.CapabilityShortcuts, api.CapabilityShortcutsWrite))
	requireCode(t, serveAliasWrite(writer, protocol.MethodRun, protocol.OpAddAlias, protocol.AliasArgs{Alias: "jot"}), api.CodeInvalid, "action id")
	requireCode(t, serveAliasWrite(writer, protocol.MethodRun, protocol.OpRemoveAlias, protocol.AliasArgs{}), api.CodeInvalid, "alias")
}

func TestAliasWritesAreUnavailableWithoutAnEditor(t *testing.T) {
	writer := New(Deps{}, manifestFor("acme.writer", api.CapabilityShortcuts, api.CapabilityShortcutsWrite))
	requireCode(t, serveAliasWrite(writer, protocol.MethodRun, protocol.OpRemoveAlias, protocol.AliasArgs{Alias: "jot"}), api.CodeUnavailable, "aliases")
}
