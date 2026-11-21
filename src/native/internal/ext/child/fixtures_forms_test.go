package child

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// runFixtureCommand runs one action of a fixture and returns the result the host would receive.
func runFixtureCommand(t *testing.T, fixture, command string) api.Result {
	t.Helper()
	_, responses := exchange(t, fixture, protocol.Request{ID: 1, Method: protocol.MethodRun, Invocation: &api.Invocation{Ref: api.ActionRef{ID: command}}})
	if len(responses) != 1 || responses[0].Failure != nil {
		t.Fatalf("%s/%s: responses = %+v; want one success", fixture, command, responses)
	}
	var result api.Result
	if err := json.Unmarshal(responses[0].Data, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return result
}

func TestFormFixtureDeclaresEverySixteenFieldType(t *testing.T) {
	result := runFixtureCommand(t, "acme.form", "form")
	if result.View == nil || result.View.Kind != api.ViewForm || result.View.Form == nil {
		t.Fatalf("view = %+v; want a form", result.View)
	}
	seen := map[api.FieldType]bool{}
	for _, field := range result.View.Form.Fields {
		seen[field.Type] = true
	}
	for _, fieldType := range api.AllFieldTypes() {
		if !seen[fieldType] {
			t.Fatalf("the form fixture has no %s field", fieldType)
		}
	}
	if result.View.Form.Submit == nil || result.View.Form.Submit.Ref == nil || result.View.Form.Submit.Ref.ID != "save" {
		t.Fatalf("submit = %+v; want the save action", result.View.Form.Submit)
	}
	if err := result.View.Validate(); err != nil {
		t.Fatalf("the form view does not validate: %v", err)
	}
}

func TestPromptFixtureOpensItsFieldsAsAFormAction(t *testing.T) {
	result := runFixtureCommand(t, "acme.form", "prompt")
	if result.View == nil || len(result.View.Items) != 1 {
		t.Fatalf("view = %+v; want one list item that opens the prompt", result.View)
	}
	primary := result.View.Items[0].Primary
	if primary == nil || primary.Kind != api.ActionPushView || primary.View == nil {
		t.Fatalf("primary = %+v; want a push-view action", primary)
	}
	if primary.View.Kind != api.ViewForm || primary.View.Form.SubmitTitle != "Create" {
		t.Fatalf("prompt view = %+v; want a form titled to submit as Create", primary.View)
	}
	if got := primary.View.Form.Fields[0]; got.Type != api.FieldDescription || got.Description == "" {
		t.Fatalf("first prompt row = %+v; want the message", got)
	}
	if err := primary.Validate(); err != nil {
		t.Fatalf("the prompt action does not validate: %v", err)
	}
}

func TestEditorFixtureOpensWithADraftAndAConflictHandler(t *testing.T) {
	result := runFixtureCommand(t, "acme.editor", "note")
	if result.View == nil || result.View.Kind != api.ViewEditor || result.View.Editor == nil {
		t.Fatalf("view = %+v; want an editor", result.View)
	}
	draft := result.View.Editor.Draft
	if draft == nil || draft.Key != "note-1" || draft.Version != "v1" || draft.OnConflict == nil || draft.OnConflict.ID != "resolve" {
		t.Fatalf("draft = %+v; want note-1 at v1 with a resolve handler", draft)
	}
}

func TestPickerFixtureDeclaresFileFilesAndFolderFields(t *testing.T) {
	result := runFixtureCommand(t, "acme.picker", "pick")
	if result.View == nil || result.View.Form == nil {
		t.Fatalf("view = %+v; want a form", result.View)
	}
	want := []api.FieldType{api.FieldFile, api.FieldFiles, api.FieldFolder}
	for i, field := range result.View.Form.Fields {
		if field.Type != want[i] {
			t.Fatalf("field %d = %s; want %s", i, field.Type, want[i])
		}
	}
}
