package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

type shortcutRunProbe struct {
	result        api.Result
	failure       *api.Failure
	err, pasteErr error
	invocation    api.Invocation
	extID         string
	actions       []api.Action
	hidden        bool
}

func (r *shortcutRunProbe) Run(_ context.Context, id string, invocation api.Invocation) (api.Result, *api.Failure, error) {
	r.extID, r.invocation = id, invocation
	return r.result, r.failure, r.err
}

func (r *shortcutRunProbe) PerformPasteForIdentity(id string, identity any, action api.Action) error {
	if identity != r.result.OriginIdentity {
		return errors.New("paste lost captured origin")
	}
	if !r.hidden || id != r.extID {
		return errors.New("paste lost its dismissal or extension identity")
	}
	r.actions = append(r.actions, action)
	return r.pasteErr
}

func TestGlobalBackgroundShortcutExecutesItsDeclarativePaste(t *testing.T) {
	for _, action := range []api.Action{api.PasteText("Paste Dictation", "transcript"), {Kind: api.ActionPaste}, {Kind: api.ActionTypeText, Text: "transcript"}} {
		t.Run(string(action.Kind), func(t *testing.T) {
			runner := &shortcutRunProbe{result: api.Result{Next: &action, OriginBound: true}}
			entry := extensionCommandEntry{originBound: true, extID: "nevermind.dictation", command: api.Command{ID: "dictate", Mode: api.ModeBackground}}
			result, failure, err := runShortcutCommand(context.Background(), runner, entry, func() error { runner.hidden = true; return nil })
			if err != nil || failure != nil || result.Next != nil || len(runner.actions) != 1 {
				t.Fatalf("shortcut dropped its follow-up: result=%+v failure=%v err=%v actions=%v", result, failure, err, runner.actions)
			}
			if !runner.invocation.Background || runner.invocation.Ref.ID != "dictate" || runner.actions[0].Kind != action.Kind {
				t.Fatal("shortcut changed the command lifecycle or action")
			}
		})
	}
}

func TestGlobalShortcutPreservesPasteFailuresAndSkipsFailedRuns(t *testing.T) {
	action := api.PasteText("Paste", "transcript")
	refused := errors.New("paste permission denied")
	runner := &shortcutRunProbe{result: api.Result{Next: &action, OriginBound: true}, pasteErr: refused}
	entry := extensionCommandEntry{originBound: true, extID: "acme.dictation", command: api.Command{ID: "dictate"}}
	_, _, err := runShortcutCommand(context.Background(), runner, entry, func() error { runner.hidden = true; return nil })
	if !errors.Is(err, refused) {
		t.Fatalf("paste failure = %v", err)
	}
	runner.actions = nil
	runner.failure = &api.Failure{Code: api.CodeInternal, Message: "transcription failed"}
	_, failure, _ := runShortcutCommand(context.Background(), runner, entry, func() error { t.Fatal("failed run dismissed the palette"); return nil })
	if failure != runner.failure || len(runner.actions) != 0 {
		t.Fatal("a failed transcription still pasted")
	}
}

func TestBackgroundPasteFixtureDeclaresItsShortcut(t *testing.T) {
	m := newFixtureManager(t, "acme.paste")
	waitForState(t, m, "acme.paste", supervisor.StateRunning)
	for _, result := range m.Commands(context.Background()) {
		if result.ExtensionID != "acme.paste" {
			continue
		}
		if result.Failure != nil || len(result.Commands) != 1 || result.Commands[0].GlobalShortcut == "" {
			t.Fatalf("background paste command discovery = %+v", result)
		}
		return
	}
	t.Fatal("background paste fixture did not advertise Commands")
}

func TestGlobalShortcutRejectsPasteWithoutCapturedOrigin(t *testing.T) {
	action := api.PasteText("Paste", "fixture")
	runner := &shortcutRunProbe{result: api.Result{Next: &action}}
	entry := extensionCommandEntry{originBound: true, extID: "acme", command: api.Command{ID: "run"}}
	_, _, err := runShortcutCommand(context.Background(), runner, entry, func() error { t.Fatal("unbound action hid palette"); return nil })
	if !errors.Is(err, manager.ErrChangedInstallation) || len(runner.actions) != 0 {
		t.Fatalf("err=%v actions=%v", err, runner.actions)
	}
}

func TestGlobalShortcutBindsCommandToCapturedInstallation(t *testing.T) {
	identity := new(int)
	runner := &shortcutRunProbe{}
	entry := extensionCommandEntry{extID: "acme", command: api.Command{ID: "run"}, identity: identity, originBound: true}
	if _, _, err := runShortcutCommand(context.Background(), runner, entry, func() error { t.Fatal("unexpected paste"); return nil }); err != nil {
		t.Fatal(err)
	}
	if !runner.invocation.RequireIdentity || runner.invocation.ExpectedIdentity != identity {
		t.Fatalf("invocation=%+v", runner.invocation)
	}
}
