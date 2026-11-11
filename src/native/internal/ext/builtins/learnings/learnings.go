// Package learnings is the built-in that lists the rules the AI builder learned from the user's builder chats. The AI
// Learnings command opens the list in the palette; a rule's primary action edits its text in an editor, and Delete asks
// first. Both changes are guarded by the revisions the list was read at, so a rule an automatic review changed meanwhile
// is refused with a message, never overwritten, and a deleted rule is not learned again.
//
// Layer: ext. The package imports api, search and the standard library.
package learnings

import (
	"fmt"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.ai-learnings"

// CommandID is the root command that opens the list. It doubles as the ref that opens it.
const CommandID = "ai-learnings"

// Refs of the actions the rows and the editor run. Args carry the rule id and the revisions the list was read at.
const (
	refEdit    = "edit"
	refSave    = "save"
	refDelete  = "delete"
	refRefresh = "refresh-list"
	argID      = "id"
	argList    = "listRevision"
	argRule    = "ruleRevision"
)

const (
	listTitle  = "AI Learnings"
	editTitle  = "Edit Learning"
	scoreRoot  = 12
	staleCopy  = "The learnings changed. Nothing was saved; the list shows the current rules."
	staleErase = "The learnings changed. Nothing was deleted; the list shows the current rules."
)

// Extension is the AI Learnings built-in. It has no state: the host keeps the rules.
type Extension struct{}

// New returns the built-in.
func New() Extension { return Extension{} }

// Manifest identifies the built-in.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: listTitle}}
}

// Commands lists the AI Learnings command at the root.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID: CommandID, Title: listTitle, Subtitle: "Rules the AI builder learned from you", Icon: "brain", Score: scoreRoot,
		Keywords: []string{"ai", "learnings", "rules", "memory", "builder", "preferences"}, Section: search.KindCommands,
		Appearance: &api.Appearance{Foreground: api.ColorPurple}, Customizable: true, LearnKey: "cmd:" + listTitle,
	}}, nil
}

// RunAction opens the list, the editor, and runs the save and delete actions.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case CommandID:
		list, err := ctx.Learnings().List()
		if err != nil {
			return api.Result{}, err
		}
		view := listView(list)
		return ctx.Navigation().Push(&view), nil
	case refRefresh:
		list, err := ctx.Learnings().List()
		if err != nil {
			return api.Result{}, err
		}
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: listItems(list)}}, nil
	case refEdit:
		return openEditor(ctx, in.Ref.Args)
	case refSave:
		return save(ctx, in)
	case refDelete:
		return remove(ctx, in.Ref.Args)
	}
	return api.Result{}, fmt.Errorf("the AI learnings built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// target is the rule an action names and the revisions the list was read at.
type target struct {
	id           string
	listRevision uint64
	ruleRevision uint64
}

func targetOf(args map[string]string) (target, error) {
	list, listErr := strconv.ParseUint(args[argList], 10, 64)
	rule, ruleErr := strconv.ParseUint(args[argRule], 10, 64)
	if args[argID] == "" || listErr != nil || ruleErr != nil {
		return target{}, fmt.Errorf("the learning action is incomplete: %w", api.ErrInvalidInput)
	}
	return target{id: args[argID], listRevision: list, ruleRevision: rule}, nil
}

// openEditor shows the rule's text for editing, bound to the revisions the list showed.
func openEditor(ctx api.Context, args map[string]string) (api.Result, error) {
	want, err := targetOf(args)
	if err != nil {
		return api.Result{}, err
	}
	list, err := ctx.Learnings().List()
	if err != nil {
		return api.Result{}, err
	}
	if list.Revision != want.listRevision {
		return staleList(list, staleCopy), nil
	}
	for _, rule := range list.Rules {
		if rule.ID == want.id && rule.Revision == want.ruleRevision {
			view := editorView(rule, list.Revision)
			return ctx.Navigation().Push(&view), nil
		}
	}
	return staleList(list, staleCopy), nil
}

// save stores the edited text. A stale save changes nothing and returns to the refreshed list; an invalid one keeps the
// editor open with the text the user typed.
func save(ctx api.Context, in api.Invocation) (api.Result, error) {
	want, err := targetOf(in.Ref.Args)
	if err != nil {
		return api.Result{}, err
	}
	outcome, err := ctx.Learnings().Edit(want.listRevision, want.id, want.ruleRevision, in.EditorContent)
	if err != nil {
		return api.Result{}, err
	}
	switch outcome {
	case api.LearningSaved:
		return api.Result{Navigation: api.NavPop, Toast: &api.Toast{Message: "Learning saved", Tone: "success"}}, nil
	case api.LearningInvalid:
		return api.Result{Toast: &api.Toast{Message: "Not saved: a rule needs 1 to 300 characters, with no links, paths, code or secrets.", Tone: "error"}}, nil
	}
	return api.Result{Navigation: api.NavPop, Toast: &api.Toast{Message: staleCopy, Tone: "error"}}, nil
}

// remove deletes the rule the user confirmed and shows the list without it. A stale delete changes nothing.
func remove(ctx api.Context, args map[string]string) (api.Result, error) {
	want, err := targetOf(args)
	if err != nil {
		return api.Result{}, err
	}
	outcome, err := ctx.Learnings().Delete(want.listRevision, want.id, want.ruleRevision)
	if err != nil {
		return api.Result{}, err
	}
	list, err := ctx.Learnings().List()
	if err != nil {
		return api.Result{}, err
	}
	toast := &api.Toast{Message: "Learning deleted", Tone: "success"}
	if outcome != api.LearningSaved {
		toast = &api.Toast{Message: staleErase, Tone: "error"}
	}
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: listItems(list)}, Toast: toast}, nil
}

// staleList answers an action made on a list that moved on: the current rules and the reason.
func staleList(list api.LearningList, message string) api.Result {
	return api.Result{
		Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: listItems(list)},
		Toast: &api.Toast{Message: message, Tone: "error"},
	}
}
