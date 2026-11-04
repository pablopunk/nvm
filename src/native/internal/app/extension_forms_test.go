package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// formsGoldens is the group of the form, editor and picker goldens under testdata/golden/forms.
var formsGoldens = golden.Options{Group: "forms", Width: width, Height: height}

// Fixture ids of the forms, editor and picker, each a dev fixture under internal/ext/testdata/extensions.
const (
	formFixture   = "acme.form"
	editorFixture = "acme.editor"
	pickerFixture = "acme.picker"
	noteDraftKey  = "note-1"
)

// formsManager runs the three form fixtures in their child processes.
func formsManager(t *testing.T) *manager.Manager {
	t.Helper()
	m := newFixtureManager(t, formFixture, editorFixture, pickerFixture)
	for _, id := range []string{formFixture, editorFixture, pickerFixture} {
		waitForState(t, m, id, supervisor.StateRunning)
	}
	return m
}

// fixtureSession asks a fixture for the view of ref, as the palette does when the user runs its command.
func fixtureSession(t *testing.T, m *manager.Manager, extID, ref string, deps extview.Deps) *extview.Session {
	t.Helper()
	result, failure, err := m.Run(context.Background(), extID, api.Invocation{Ref: api.ActionRef{ID: ref}})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("run %s/%s: err %v failure %+v", extID, ref, err, failure)
	}
	return extview.NewSession(deps, extID, *result.View)
}

// formDeps runs actions and drafts through the manager on the calling goroutine, and file dialogs through dialog.
func formDeps(m *manager.Manager, dialog *fake.FileDialog, ui chan func()) extview.Deps {
	deps := syncViewDeps(m)
	deps.Drafts = m
	if dialog != nil {
		deps.Dialog = dialog
	}
	if ui != nil {
		deps.OnUI = func(fn func()) { ui <- fn }
	}
	return deps
}

// settleDialog runs the UI callback a picker posted, on the test's goroutine.
func settleDialog(t *testing.T, ui chan func()) {
	t.Helper()
	(<-ui)()
}

// fillRow runs the edit action of a field row and then types text into the search field, as the user does.
func editField(t *testing.T, page *palette.View, h *viewHost, title string) {
	t.Helper()
	row := findRowTitled(t, page.Search(""), title)
	actionTitled(t, row, "Edit").Run(h)
}

func setField(t *testing.T, page *palette.View, h *viewHost, title, text string) {
	t.Helper()
	editField(t, page, h, title)
	actionTitled(t, findRowTitled(t, page.Search(text), "Set "+title), "Set").Run(h)
}

func submitForm(t *testing.T, page *palette.View, h *viewHost, title string) {
	t.Helper()
	actionTitled(t, findRowTitled(t, page.Search(""), title), "Create").Run(h)
}

// TestFormFixtureSubmitRunsThroughTheChild sets the required fields of the form fixture and submits it. The submit
// reaches the child, which answers with the values it received as a preview page.
func TestFormFixtureSubmitRunsThroughTheChild(t *testing.T) {
	m := formsManager(t)
	session := fixtureSession(t, m, formFixture, "form", formDeps(m, nil, nil))
	page := session.Palette()
	h := &viewHost{}

	setField(t, page, h, "Name", "Ada Lovelace")
	editField(t, page, h, "Accept the terms")
	submitForm(t, page, h, "Create")

	if len(h.pushed) != 1 || pushedTitle(h) != "Submitted" {
		t.Fatalf("pushed = %d views; want the submitted preview from the child", len(h.pushed))
	}
}

// TestFormFixtureRefusesAMissingRequiredFieldBeforeTheChild keeps the submit away from the child when the host's own
// validator refuses the form.
func TestFormFixtureRefusesAMissingRequiredFieldBeforeTheChild(t *testing.T) {
	m := formsManager(t)
	session := fixtureSession(t, m, formFixture, "form", formDeps(m, nil, nil))
	page := session.Palette()
	h := &viewHost{}

	submitForm(t, page, h, "Create")
	if len(h.pushed) != 0 {
		t.Fatal("a form with a missing required field reached the child")
	}
	if got := findRowTitled(t, page.Search(""), "Name").Subtitle; got != "Name is required" {
		t.Fatalf("name subtitle = %q; want the required copy", got)
	}
}

// TestDraftSurvivesARestartAndCommitClearsIt keeps a draft in the host store, opens the editor over it, and then has
// the extension commit: the child's Drafts().Commit removes the draft from the same store.
func TestDraftSurvivesARestartAndCommitClearsIt(t *testing.T) {
	m := formsManager(t)
	if err := m.SaveDraft(editorFixture, protocol.DraftSaveArgs{Key: noteDraftKey, Version: "v1", Content: "Typed before the crash"}); err != nil {
		t.Fatal(err)
	}
	fixtureSession(t, m, editorFixture, "note", formDeps(m, nil, nil)).Palette()
	if lookup, err := m.LoadDraft(editorFixture, noteDraftKey); err != nil || !lookup.Found {
		t.Fatalf("opening the note discarded its draft: %+v, %v", lookup, err)
	}

	result, failure, err := m.Run(context.Background(), editorFixture, api.Invocation{Ref: api.ActionRef{ID: "commit"}, EditorContent: "Typed before the crash"})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("commit: err %v failure %+v", err, failure)
	}
	if lookup, _ := m.LoadDraft(editorFixture, noteDraftKey); lookup.Found {
		t.Fatal("the child's commit left the draft in the store")
	}
}

// TestConflictingDraftIsResolvedByTheExtension opens the note over a draft of another version. The handler resets the
// draft, so the store no longer holds it.
func TestConflictingDraftIsResolvedByTheExtension(t *testing.T) {
	m := formsManager(t)
	if err := m.SaveDraft(editorFixture, protocol.DraftSaveArgs{Key: noteDraftKey, Version: "v0", Content: "Old draft"}); err != nil {
		t.Fatal(err)
	}
	fixtureSession(t, m, editorFixture, "note", formDeps(m, nil, nil)).Palette()
	if lookup, _ := m.LoadDraft(editorFixture, noteDraftKey); lookup.Found {
		t.Fatal("the reset resolution kept the stored draft")
	}
}

// TestPickerFixtureStoresTheDialogsPicks sends the file and folder rows through the native dialog port and submits
// what the dialog returned.
func TestPickerFixtureStoresTheDialogsPicks(t *testing.T) {
	m := formsManager(t)
	dialog := fake.NewFileDialog()
	ui := make(chan func(), 8)
	session := fixtureSession(t, m, pickerFixture, "pick", formDeps(m, dialog, ui))
	page := session.Palette()
	h := &viewHost{}
	dialog.Answer("/pictures/ada.png")
	dialog.Answer("/docs/a.txt", "/docs/b.txt")
	dialog.Answer("/backups")

	for _, title := range []string{"Avatar", "Attachments", "Backup folder"} {
		actionTitled(t, findRowTitled(t, page.Search(""), title), "Edit").Run(h)
		settleDialog(t, ui)
	}
	actionTitled(t, findRowTitled(t, page.Search(""), "Save"), "Save").Run(h)
	if len(h.pushed) != 1 || pushedTitle(h) != "Picked" {
		t.Fatalf("pushed = %d views; want the picked preview", len(h.pushed))
	}
}

// goldenFormPage opens a fixture view in the golden palette and returns its page and the host it acts through.
func goldenFormPage(t *testing.T, p *App, m *manager.Manager, extID, ref string, deps extview.Deps) (*palette.View, *viewHost) {
	t.Helper()
	page := fixtureSession(t, m, extID, ref, deps).Open(&viewHost{})
	h := &viewHost{}
	goldenOpen(p, page)
	return page, h
}

// TestGoldenFormRowsAtRest draws every field of the form fixture, untouched.
func TestGoldenFormRowsAtRest(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	goldenFormPage(t, p, m, formFixture, "form", formDeps(m, nil, nil))
	golden.AssertMatrix(t, "form-rows", p.View, formsGoldens)
}

// TestGoldenFormValidationErrors draws the form after a submit that the host refused.
func TestGoldenFormValidationErrors(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	page, h := goldenFormPage(t, p, m, formFixture, "form", formDeps(m, nil, nil))
	submitForm(t, page, h, "Create")
	goldenQuery(p, "")
	golden.AssertMatrix(t, "form-errors", p.View, formsGoldens)
}

// TestGoldenFormEditingText draws a text field while its value is edited in the search field.
func TestGoldenFormEditingText(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	page, h := goldenFormPage(t, p, m, formFixture, "form", formDeps(m, nil, nil))
	editField(t, page, h, "Name")
	goldenQuery(p, "Ada Lovelace")
	golden.AssertMatrix(t, "form-editing-text", p.View, formsGoldens)
}

// TestGoldenFormChoiceAndMultiselect draws a dropdown's options and a multiselect with one option on.
func TestGoldenFormChoiceAndMultiselect(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	page, h := goldenFormPage(t, p, m, formFixture, "form", formDeps(m, nil, nil))
	editField(t, page, h, "Plan")
	goldenQuery(p, "")
	golden.AssertMatrix(t, "form-editing-choice", p.View, formsGoldens)

	actionTitled(t, findRowTitled(t, page.Search(""), "Pro"), "Pro").Run(h)
	editField(t, page, h, "Tags")
	actionTitled(t, findRowTitled(t, page.Search(""), "Bug"), "Bug").Run(h)
	goldenQuery(p, "")
	golden.AssertMatrix(t, "form-editing-multiselect", p.View, formsGoldens)
}

// TestGoldenEditorRecoversADraft draws the note editor with a draft from before a crash.
func TestGoldenEditorRecoversADraft(t *testing.T) {
	m := formsManager(t)
	if err := m.SaveDraft(editorFixture, protocol.DraftSaveArgs{Key: noteDraftKey, Version: "v1", Content: "Typed before the crash"}); err != nil {
		t.Fatal(err)
	}
	p := goldenPalette(t)
	goldenFormPage(t, p, m, editorFixture, "note", formDeps(m, nil, nil))
	golden.AssertMatrix(t, "editor-recovered", p.View, formsGoldens)
}

// TestGoldenPickerAfterPicks draws the picker fixture with every path chosen.
func TestGoldenPickerAfterPicks(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	dialog := fake.NewFileDialog()
	ui := make(chan func(), 8)
	page, h := goldenFormPage(t, p, m, pickerFixture, "pick", formDeps(m, dialog, ui))
	dialog.Answer("/pictures/ada.png")
	dialog.Answer("/docs/a.txt", "/docs/b.txt")
	dialog.Answer("/backups")
	for _, title := range []string{"Avatar", "Attachments", "Backup folder"} {
		actionTitled(t, findRowTitled(t, page.Search(""), title), "Edit").Run(h)
		settleDialog(t, ui)
	}
	goldenQuery(p, "")
	golden.AssertMatrix(t, "picker-picked", p.View, formsGoldens)
}

// pushedTitle is the title of the last view the host was asked to push.
func pushedTitle(h *viewHost) string {
	view, ok := h.pushed[len(h.pushed)-1].(*palette.View)
	if !ok {
		return ""
	}
	return view.Title
}

// TestGoldenFormEditingPassword draws a password being typed: the search field masks the text, so the golden never
// shows the secret. The secret typed here is a stand-in that appears nowhere else in the golden.
func TestGoldenFormEditingPassword(t *testing.T) {
	m := formsManager(t)
	p := goldenPalette(t)
	page, h := goldenFormPage(t, p, m, formFixture, "form", formDeps(m, nil, nil))
	editField(t, page, h, "API token")
	goldenQuery(p, "hunter2-stand-in")
	golden.AssertMatrix(t, "form-editing-password", p.View, formsGoldens)
}
