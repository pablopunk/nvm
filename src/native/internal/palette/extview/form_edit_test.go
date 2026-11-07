package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// editWindow is the palette's window in these tests: updates run at once and nothing is shown.
type editWindow struct{}

func (editWindow) Update(fn func())     { fn() }
func (editWindow) Show()                {}
func (editWindow) Hide()                {}
func (editWindow) Focus()               {}
func (editWindow) IsVisible() bool      { return true }
func (editWindow) IsFocused() bool      { return true }
func (editWindow) Size() (int, int)     { return 720, 470 }
func (editWindow) SetPosition(x, y int) {}

// formInPalette runs a form page in a real palette, so the text and the caret of the search field are the palette's.
type formInPalette struct {
	t  *testing.T
	r  *formRig
	p  *palette.Palette
	tt *ui.Tester
}

func openFormInPalette(t *testing.T, spec api.FormSpec) *formInPalette {
	t.Helper()
	r := openForm(t, spec)
	p := palette.New(palette.Options{Root: r.page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	return &formInPalette{t: t, r: r, p: p, tt: tt}
}

func (f *formInPalette) key(mods ui.Modifiers, key ui.Key) {
	f.tt.Key(mods, key)
	f.tt.Frame()
}

func (f *formInPalette) typeText(s string) {
	f.tt.Type(s)
	f.tt.Frame()
}

func (f *formInPalette) expectQuery(want string) {
	f.t.Helper()
	if got := f.p.Query(); got != want {
		f.t.Fatalf("the search field holds %q; want %q", got, want)
	}
}

func (f *formInPalette) expectValue(id, want string) {
	f.t.Helper()
	if got := f.r.session.form.values[id].Text; got != want {
		f.t.Fatalf("field %s = %q; want %q", id, got, want)
	}
}

func (f *formInPalette) expectSubmitted(id, want string) {
	f.t.Helper()
	if len(f.r.invokes) == 0 {
		f.t.Fatal("the submit action did not run")
	}
	if got := f.r.lastInvoke().Form[id].Text; got != want {
		f.t.Fatalf("submitted %s = %q; want %q", id, got, want)
	}
}

func prefilledName(value string) api.Field {
	name := textField("name", "Name")
	name.Value = api.TextValue(value)
	return name
}

func TestPrefilledFieldEditTypesAfterItsValue(t *testing.T) {
	f := openFormInPalette(t, api.FormSpec{Fields: []api.Field{prefilledName("A")}})

	f.key(0, ui.KeyEnter)
	f.expectQuery("A")
	f.typeText("da Lovelace")
	f.expectQuery("Ada Lovelace")
	f.key(0, ui.KeyEnter)
	f.expectValue("name", "Ada Lovelace")
}

func TestAppendingToAPrefilledFieldKeepsItsValueAndAddsAtTheEnd(t *testing.T) {
	f := openFormInPalette(t, api.FormSpec{Fields: []api.Field{prefilledName("Ada")}})

	f.key(0, ui.KeyEnter)
	f.typeText(" Lovelace")
	f.key(0, ui.KeyEnter)
	f.key(0, ui.KeyEnter)
	f.expectQuery("Ada Lovelace")
	f.typeText("!")
	f.key(0, ui.KeyEnter)
	f.expectValue("name", "Ada Lovelace!")
}

func TestClearingAPrefilledFieldAndRetypingReplacesIt(t *testing.T) {
	f := openFormInPalette(t, api.FormSpec{Fields: []api.Field{prefilledName("Ada")}})

	f.key(0, ui.KeyEnter)
	for range 3 {
		f.key(0, ui.KeyBackspace)
	}
	f.expectQuery("")
	f.typeText("Grace")
	f.key(0, ui.KeyEnter)
	f.expectValue("name", "Grace")
}

// TestValidationRetryEditsTheRejectedValueAtItsEnd is the silent wrong-data case: a rejected email is edited, its text
// is erased with Backspace, and the corrected address is typed. The submit must carry the corrected address alone.
func TestValidationRetryEditsTheRejectedValueAtItsEnd(t *testing.T) {
	email := api.FieldOf("email", api.FieldEmail, "Email")
	email.Value = api.TextValue("not-an-email")
	f := openFormInPalette(t, api.FormSpec{Fields: []api.Field{email}, Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})

	f.key(palette.Mod, ui.KeyEnter)
	if len(f.r.invokes) != 0 {
		t.Fatal("an invalid email was submitted")
	}
	f.key(0, ui.KeyEnter)
	f.expectQuery("not-an-email")
	for range len("not-an-email") {
		f.key(0, ui.KeyBackspace)
	}
	f.typeText("ada@example.test")
	f.key(0, ui.KeyEnter)
	f.key(palette.Mod, ui.KeyEnter)
	f.expectSubmitted("email", "ada@example.test")
}

func TestFormShortcutSavesTheCurrentDraftAndKeepsItsCTAVisible(t *testing.T) {
	f := openFormInPalette(t, api.FormSpec{Fields: []api.Field{prefilledName("Ada")}, SubmitTitle: "Create Ticket",
		Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})
	row := f.r.row("Create Ticket")
	if !row.Prominent || row.Tone != "yellow" || row.Accessory != palette.KeyLabel("⌃↵") {
		t.Fatalf("form submit is not presented as a CTA: %+v", row)
	}
	if hint := f.r.page.Primary(); hint.Title != "Create Ticket" || hint.Key != palette.KeyLabel("⌃↵") {
		t.Fatalf("persistent form action = %+v", hint)
	}
	f.key(0, ui.KeyEnter)
	f.typeText(" Lovelace")
	f.key(palette.Mod, ui.KeyEnter)
	f.expectSubmitted("name", "Ada Lovelace")
}
