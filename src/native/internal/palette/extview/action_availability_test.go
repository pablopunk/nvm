package extview

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func withoutNativePreview(action api.Action) bool { return action.Kind != api.ActionQuickLook }

func TestUnsupportedPrimaryIsNotReplacedBySecondaryAction(t *testing.T) {
	primary := api.QuickLook("Preview", "/tmp/a.pdf")
	item := api.Item{ID: "file", Title: "a.pdf", Primary: &primary, Actions: []api.Action{api.Run("remove", "Remove", nil)}}
	original := item
	session := NewSession(Deps{ActionSupported: withoutNativePreview}, "acme.files", api.ListView("Files", item))
	row := session.row(item)
	if _, ok := row.DefaultAction(); ok {
		t.Fatal("secondary action became Enter after preview was omitted")
	}
	if len(row.Actions) != 1 || row.Actions[0].Title != "Remove" || row.Title != "a.pdf" {
		t.Fatalf("row=%+v", row)
	}
	if !reflect.DeepEqual(item, original) {
		t.Fatal("availability projection changed source item")
	}
	root := search.Item{Actions: MenuActionsFor(Deps{ActionSupported: withoutNativePreview}, "acme.files", MenuSections(item.Primary, item.ActionPanel, item.Actions))}
	if _, ok := root.DefaultAction(); ok {
		t.Fatal("root promoted secondary action to Enter")
	}
}

func TestUnsupportedSecondaryKeepsPrimaryAndContent(t *testing.T) {
	primary := api.OpenPath("Open", "/tmp/a.pdf")
	preview := api.QuickLook("Preview", "/tmp/a.pdf")
	panel := api.Panel(api.Section("Preview", preview), api.Section("Other", api.Run("copy", "Copy", nil)))
	item := api.Item{ID: "file", Title: "a.pdf", Primary: &primary, ActionPanel: panel}
	session := NewSession(Deps{ActionSupported: withoutNativePreview}, "acme.files", api.ListView("Files", item))
	row := session.row(item)
	action, ok := row.DefaultAction()
	if !ok || action.Title != "Open" || len(row.Actions) != 2 {
		t.Fatalf("actions=%+v", row.Actions)
	}
	if len(panel.Sections) != 2 || len(panel.Sections[0].Actions) != 1 {
		t.Fatal("projection mutated original action panel")
	}
}

func TestUnknownSupportPreservesAction(t *testing.T) {
	preview := api.QuickLook("Preview", "/tmp/a.pdf")
	actions := MenuActionsFor(Deps{}, "acme.files", MenuSections(&preview, nil, nil))
	if len(actions) != 1 {
		t.Fatal("unknown support removed a visible action")
	}
}
