// Package aicommands is the built-in that corrects the selected text with AI, and undoes the last correction. It is
// an ordinary extension in every way but its place: it uses only the public API, so the selection, the model, the
// indicator and the undo are the calls any extension can make.
//
// Layer: ext. The package imports api, ai/commands, search and the standard library.
package aicommands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. It is the 1.x ai-commands extension id, so the command's shortcut keeps its 1.x key.
const ID = "nevermind.ai-commands"

// FixCommandID is the command that corrects the selection. Its global id is "extension:<ID>:<FixCommandID>", the key
// the 1.x app stored the shortcut under.
const FixCommandID = "fix-selected-text-with-ai"

// UndoID is the row and action that put the last correction back.
const UndoID = "undo-fix"

// FixTitle and UndoTitle are the titles the palette and the indicator show.
const (
	FixTitle  = "Fix Selected Text with AI"
	UndoTitle = "Undo Fix"
)

// selectionCopyCapability is the public name of the capability the fix needs: sending the copy gesture.
const selectionCopyCapability = "selection.copy"

// Extension is the AI commands built-in. Its fixLog remembers the last correction so Undo can reach it.
type Extension struct {
	fixes *fixLog
}

// New returns the built-in with an empty undo record.
func New() Extension {
	return Extension{fixes: &fixLog{}}
}

// Manifest identifies the built-in and declares the capabilities its calls need, as a folder extension would.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: ID, Title: "AI Commands"},
		Capabilities: []api.Capability{
			api.CapabilityAI, api.CapabilityDesktopSelection, api.CapabilityDesktopKeyboard,
		},
	}
}

// Commands lists the fix and the triage when the OS can copy the selection. Without that capability the commands do not
// exist, so the palette never offers one that cannot run.
func (Extension) Commands(ctx api.Context) ([]api.Command, error) {
	if !ctx.System().HasCapability(selectionCopyCapability) {
		return nil, nil
	}
	return []api.Command{{
		ID: FixCommandID, Title: FixTitle, Subtitle: "Correct grammar, spelling, and punctuation", Icon: "sparkles",
		Aliases:         []string{"fix grammar", "correct selected text", "proofread"},
		Keywords:        []string{"fix grammar correct proofread spelling selected text ai"},
		Mode:            api.ModeBackground,
		DismissAfterRun: true,

		Placement: []api.Placement{api.PlacementRoot, api.PlacementSearch},
	}, triageCommand()}, nil
}

// RootItems lists the undo row at the empty root while a fix can still be undone.
func (e Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	if !e.fixes.pending(ctx.Clock().Now()) {
		return nil, nil
	}
	return []api.Item{undoItem(1000)}, nil
}

// SearchItems lists the undo row for a query that names it, while a fix can still be undone.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" || !strings.Contains(strings.ToLower(UndoTitle), needle) {
		return nil, nil
	}
	if !e.fixes.pending(ctx.Clock().Now()) {
		return nil, nil
	}
	return []api.Item{undoItem(100)}, nil
}

// undoItem is the row that runs UndoID.
func undoItem(score float64) api.Item {
	return api.Item{
		ID: UndoID, Title: UndoTitle, Subtitle: "Put the text back as it was before the fix", Icon: "undo-2", Score: score,
		Section: search.KindCommands, LearnKey: "cmd:" + UndoTitle,
		Primary: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: UndoID}, Title: UndoTitle, DismissAfterRun: true, Background: true},
	}
}

// RunAction runs the fix or the undo.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case FixCommandID:
		return api.Result{}, e.fix(ctx)
	case TriageCommandID:
		return api.Result{}, e.triage(ctx)
	case UndoID:
		return api.Result{}, e.undo(ctx)
	}
	return api.Result{}, fmt.Errorf("aicommands has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// errNoSelection is the failure of a fix that found no text.
//
//lint:ignore ST1005 user-facing copy, shown as written
var errNoSelection = errors.New("Could not read selected text. Select it and try again")

// errNotSignedIn is the failure of a fix started without an account.
//
//lint:ignore ST1005 user-facing copy, shown as written
var errNotSignedIn = errors.New("Sign in to use AI commands.")
