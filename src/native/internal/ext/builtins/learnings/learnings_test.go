package learnings

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeLearnings is an in-memory rule list with the same revision guards as the host's.
type fakeLearnings struct {
	list    api.LearningList
	invalid bool
}

func (f *fakeLearnings) List() (api.LearningList, error) { return f.list, nil }

func (f *fakeLearnings) Edit(listRevision uint64, id string, ruleRevision uint64, text string) (api.LearningOutcome, error) {
	if f.invalid {
		return api.LearningInvalid, nil
	}
	for i, rule := range f.list.Rules {
		if rule.ID == id && listRevision == f.list.Revision && rule.Revision == ruleRevision {
			f.list.Rules[i].Text, f.list.Rules[i].Revision = text, rule.Revision+1
			f.list.Revision++
			return api.LearningSaved, nil
		}
	}
	return api.LearningStale, nil
}

func (f *fakeLearnings) Delete(listRevision uint64, id string, ruleRevision uint64) (api.LearningOutcome, error) {
	for i, rule := range f.list.Rules {
		if rule.ID == id && listRevision == f.list.Revision && rule.Revision == ruleRevision {
			f.list.Rules = append(f.list.Rules[:i], f.list.Rules[i+1:]...)
			f.list.Revision++
			return api.LearningSaved, nil
		}
	}
	return api.LearningStale, nil
}

type fakeNavigation struct{ api.Navigation }

func (fakeNavigation) Push(view *api.View) api.Result { return api.Result{View: view} }

type fakeContext struct {
	api.Context
	store *fakeLearnings
}

func (c fakeContext) Learnings() api.Learnings     { return c.store }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }

func contextWith(rules ...api.LearningRule) fakeContext {
	return fakeContext{store: &fakeLearnings{list: api.LearningList{Revision: 1, Rules: rules}}}
}

func rule(id, text string) api.LearningRule {
	return api.LearningRule{ID: id, Kind: "preference", Text: text, Revision: 1}
}

func run(t *testing.T, ctx fakeContext, action api.Action, editor string) api.Result {
	t.Helper()
	result, err := Extension{}.RunAction(ctx, api.Invocation{Ref: *action.Ref, EditorContent: editor})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func openList(t *testing.T, ctx fakeContext) api.View {
	t.Helper()
	result, err := Extension{}.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: CommandID}})
	if err != nil || result.View == nil {
		t.Fatalf("open = %+v, %v", result, err)
	}
	return *result.View
}

func TestRootCommandOpensListOfRules(t *testing.T) {
	commands, _ := Extension{}.Commands(nil)
	if len(commands) != 1 || commands[0].Title != "AI Learnings" || commands[0].GlobalShortcut != "" {
		t.Fatalf("commands = %+v", commands)
	}
	view := openList(t, contextWith(rule("a", "Prefer clear labels"), rule("b", "Use metric units")))
	if len(view.Items) != 2 || view.Items[0].Title != "Prefer clear labels" || view.Items[0].Subtitle != "preference" {
		t.Fatalf("items = %+v", view.Items)
	}
	if view.EmptyView == nil {
		t.Fatal("the list has no empty state")
	}
	if view.Items[0].Primary.Shortcut != "" || view.Items[0].Actions[0].Shortcut != "" {
		t.Fatal("a default shortcut was added")
	}
}

func TestEditSavesThroughRevisionGuard(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"))
	view := openList(t, ctx)
	edit := run(t, ctx, *view.Items[0].Primary, "")
	if edit.View == nil || edit.View.Editor == nil || !strings.Contains(edit.View.Editor.Content, "Prefer clear labels") {
		t.Fatalf("editor = %+v", edit.View)
	}
	saved := run(t, ctx, *edit.View.Editor.Submit, "Prefer short labels")
	if saved.Navigation != api.NavPop || saved.Toast.Tone != "success" || ctx.store.list.Rules[0].Text != "Prefer short labels" {
		t.Fatalf("saved = %+v, rules = %+v", saved, ctx.store.list.Rules)
	}
}

func TestStaleEditIsRefusedWithMessageAndKeepsStoredText(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"))
	view := openList(t, ctx)
	editor := run(t, ctx, *view.Items[0].Primary, "").View.Editor
	// An automatic review changes the rule while the editor is open.
	ctx.store.list.Rules[0].Text, ctx.store.list.Rules[0].Revision = "Reviewed text", 2
	ctx.store.list.Revision = 2
	result := run(t, ctx, *editor.Submit, "Typed over it")
	if result.Toast == nil || result.Toast.Tone != "error" || !strings.Contains(result.Toast.Message, "changed") {
		t.Fatalf("no clear refusal: %+v", result.Toast)
	}
	if ctx.store.list.Rules[0].Text != "Reviewed text" {
		t.Fatal("the stale edit overwrote the rule")
	}
}

func TestInvalidEditKeepsEditorOpen(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"))
	editor := run(t, ctx, *openList(t, ctx).Items[0].Primary, "").View.Editor
	ctx.store.invalid = true
	result := run(t, ctx, *editor.Submit, "see https://example.com")
	if result.Navigation == api.NavPop || result.Toast == nil || result.Toast.Tone != "error" {
		t.Fatalf("result = %+v", result)
	}
}

func TestOpeningEditorOnAStaleRowShowsCurrentList(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"))
	view := openList(t, ctx)
	ctx.store.list = api.LearningList{Revision: 2}
	result := run(t, ctx, *view.Items[0].Primary, "")
	if result.View != nil || result.Patch == nil || len(result.Patch.Items) != 0 || result.Toast.Tone != "error" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDeleteAsksFirstAndRemovesTheRule(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"), rule("b", "Use metric units"))
	remove := openList(t, ctx).Items[0].Actions[0]
	if remove.Confirm == nil || !remove.Confirm.Destructive {
		t.Fatalf("delete does not ask: %+v", remove)
	}
	result := run(t, ctx, remove, "")
	if len(result.Patch.Items) != 1 || result.Patch.Items[0].ID != "b" || result.Toast.Tone != "success" {
		t.Fatalf("result = %+v", result)
	}
}

func TestStaleDeleteChangesNothingAndSaysSo(t *testing.T) {
	ctx := contextWith(rule("a", "Prefer clear labels"))
	remove := openList(t, ctx).Items[0].Actions[0]
	ctx.store.list.Revision = 2
	result := run(t, ctx, remove, "")
	if len(ctx.store.list.Rules) != 1 || result.Toast.Tone != "error" || !strings.Contains(result.Toast.Message, "Nothing was deleted") {
		t.Fatalf("result = %+v, rules = %+v", result, ctx.store.list.Rules)
	}
}

func TestEmptyListKeepsEmptyStateOnlyInTheList(t *testing.T) {
	view := openList(t, contextWith())
	if len(view.Items) != 0 || view.EmptyView == nil || view.EmptyView.Title == "" {
		t.Fatalf("view = %+v", view)
	}
}
