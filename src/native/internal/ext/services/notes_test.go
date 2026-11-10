package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

// notesHandler is the services of an extension with the notes capability over a fresh notes store. confirm answers
// the delete confirmation.
func notesHandler(t *testing.T, r rig, confirm bool) (*Handler, *notes.Store) {
	t.Helper()
	ports := real.NewPorts()
	st, err := store.Open(ports.FS, r.clock, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	notesStore := notes.New(st, r.clock, false)
	handler := New(Deps{
		Clock:    r.clock,
		DataRoot: r.root,
		Logs:     r.logs,
		Notes:    func() *notes.Store { return notesStore },
		Confirm: func(context.Context, Question) (bool, error) {
			return confirm, nil
		},
	}, manifestFor("nevermind.notes", api.CapabilityNotes))
	t.Cleanup(func() { _ = handler.Close() })
	return handler, notesStore
}

// serveFromRun sends one call from a palette action, which is where a destructive call may run.
func serveFromRun(t *testing.T, h *Handler, op string, args any) protocol.ServiceReply {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	ctx := protocol.WithRequestMethod(context.Background(), protocol.MethodRun)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceNotes, Op: op, Args: raw})
}

func decodeNote(t *testing.T, reply protocol.ServiceReply) api.Note {
	t.Helper()
	var n api.Note
	if err := json.Unmarshal(mustOK(t, reply), &n); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	return n
}

func TestNotesCreateListGetAndUpdateInPlace(t *testing.T) {
	h, _ := notesHandler(t, newRig(t), true)
	created := decodeNote(t, serve(t, h, protocol.ServiceNotes, protocol.OpCreate, nil))

	var all []api.Note
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceNotes, protocol.OpList, nil)), &all); err != nil || len(all) != 1 {
		t.Fatalf("list = %+v, %v; want the created note", all, err)
	}

	var update protocol.NoteUpdateResult
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceNotes, protocol.OpUpdate,
		protocol.NoteUpdateArgs{ID: created.ID, Markdown: "# Plan\n\nShip it", SavedAt: created.UpdatedAt})), &update); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if update.Conflict || update.Note.Markdown != "# Plan\n\nShip it" {
		t.Fatalf("update = %+v; want the text saved without a conflict", update)
	}

	var got protocol.NoteGetResult
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceNotes, protocol.OpGet, protocol.KeyArgs{Key: created.ID})), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if !got.Found || got.Note.Markdown != "# Plan\n\nShip it" {
		t.Fatalf("get = %+v; want the saved text", got)
	}
}

func TestNotesUpdateOverANewerSavedVersionIsAConflict(t *testing.T) {
	h, store := notesHandler(t, newRig(t), true)
	created := decodeNote(t, serve(t, h, protocol.ServiceNotes, protocol.OpCreate, nil))
	if _, _, err := store.SaveIfUnchanged(created.ID, "saved elsewhere", created.UpdatedAt, created.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var update protocol.NoteUpdateResult
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceNotes, protocol.OpUpdate,
		protocol.NoteUpdateArgs{ID: created.ID, Markdown: "typed on the old version", SavedAt: created.UpdatedAt})), &update); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if !update.Conflict || update.Note.Markdown != "saved elsewhere" {
		t.Fatalf("update = %+v; want the conflict with the newer saved note", update)
	}
	if n, _ := store.Get(created.ID); n.Markdown != "saved elsewhere" {
		t.Fatalf("stored text = %q; the refused save must not write", n.Markdown)
	}
}

func TestNotesNeedTheCapability(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.plain"))
	failure := failureCode(t, serve(t, h, protocol.ServiceNotes, protocol.OpList, nil))
	if failure.Code != api.CodePermission {
		t.Fatalf("code = %q; want a permission failure for an undeclared notes capability", failure.Code)
	}
}

func TestNotesDeleteRunsOnlyWhenConfirmed(t *testing.T) {
	h, store := notesHandler(t, newRig(t), false)
	created := decodeNote(t, serve(t, h, protocol.ServiceNotes, protocol.OpCreate, nil))
	if reply := serveFromRun(t, h, protocol.OpDelete, protocol.KeyArgs{Key: created.ID}); reply.Failure == nil {
		t.Fatal("delete ran without the user's confirmation")
	}
	if _, ok := store.Get(created.ID); !ok {
		t.Fatal("refused delete removed the note")
	}

	confirmed, _ := notesHandler(t, newRig(t), true)
	first := decodeNote(t, serve(t, confirmed, protocol.ServiceNotes, protocol.OpCreate, nil))
	mustOK(t, serveFromRun(t, confirmed, protocol.OpDelete, protocol.KeyArgs{Key: first.ID}))
}

func TestNotesAdmissionPreventsStoreEffects(t *testing.T) {
	for _, op := range []string{protocol.OpList, protocol.OpGet, protocol.OpCreate, protocol.OpRecent, protocol.OpUpdate} {
		for _, mode := range []string{"caller", "lifetime", "closed", "lookup-caller", "lookup-lifetime", "lookup-closed"} {
			t.Run(op+"-"+mode, func(t *testing.T) {
				h, st := notesHandler(t, newRig(t), true)
				original, err := st.Create(h.deps.Clock.Now())
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				lifetime, stop := context.WithCancel(context.Background())
				defer stop()
				ctx = protocol.WithLifetime(ctx, lifetime)
				invalidate := func() {
					switch mode {
					case "caller", "lookup-caller":
						cancel()
					case "lifetime", "lookup-lifetime":
						stop()
					case "closed", "lookup-closed":
						_ = h.Close()
					}
				}
				if len(mode) >= 7 && mode[:7] == "lookup-" {
					h.deps.Notes = func() *notes.Store { invalidate(); return st }
				} else {
					h.deps.Notes = func() *notes.Store { t.Fatal("lookup after cancellation"); return nil }
					invalidate()
				}
				args := mustRaw(t, protocol.NoteUpdateArgs{ID: original.ID, Markdown: "changed", SavedAt: original.UpdatedAt})
				reply := h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceNotes, Op: op, Args: args})
				want := api.CodeCancelled
				if strings.HasSuffix(mode, "closed") {
					want = api.CodeUnavailable
				}
				if reply.Failure == nil || reply.Failure.Code != want {
					t.Fatalf("failure=%+v want=%v", reply.Failure, want)
				}
				all := st.List()
				if len(all) != 1 || all[0] != original {
					t.Fatalf("store changed: %+v", all)
				}
			})
		}
	}
}
