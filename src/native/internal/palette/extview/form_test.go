package extview

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// pageHost is the palette as a form row sees it. SetQuery stores the text, as the palette's search field does; the
// test then asks the page for its rows with that text.
type pageHost struct {
	query     string
	pushed    []*palette.View
	pops      int
	hidden    bool
	refreshes int
	toasts    []string
}

func (h *pageHost) Hide()            { h.hidden = true }
func (h *pageHost) Pop()             { h.pops++ }
func (h *pageHost) Root()            {}
func (h *pageHost) Push(view any)    { h.pushed = append(h.pushed, view.(*palette.View)) }
func (h *pageHost) Toast(msg string) { h.toasts = append(h.toasts, msg) }
func (h *pageHost) Refresh()         { h.refreshes++ }
func (h *pageHost) Query() string    { return h.query }
func (h *pageHost) SetQuery(q string) {
	h.query = q
}

var _ search.Host = (*pageHost)(nil)

// formRig opens one form view in a session with fakes for the invoke, dialog and clock ports.
type formRig struct {
	t       *testing.T
	session *Session
	page    *palette.View
	host    *pageHost
	invokes []api.Invocation
	dialog  *fake.FileDialog
	ui      *uiQueue
}

func openForm(t *testing.T, spec api.FormSpec) *formRig {
	t.Helper()
	r := &formRig{t: t, host: &pageHost{}, dialog: fake.NewFileDialog(), ui: newUIQueue()}
	deps := Deps{
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			r.invokes = append(r.invokes, in)
			done(api.Result{}, nil)
		},
		Dialog: r.dialog,
		Clock:  fake.NewClock(),
		OnUI:   r.ui.post,
	}
	view := api.View{Kind: api.ViewForm, Title: "Create", Form: &spec}
	session := NewSession(deps, "acme.form", view)
	r.session = session
	r.page = session.Palette()
	return r
}

// rows answers the search for q, as the palette does after the user types q.
func (r *formRig) rows(q string) []search.Item {
	r.host.query = q
	return r.page.Search(q)
}

func (r *formRig) row(title string) search.Item {
	r.t.Helper()
	return findRow(r.t, r.rows(r.host.query), title)
}

// press runs the row's primary action, as Enter does.
func (r *formRig) press(row search.Item) {
	r.t.Helper()
	if len(row.Actions) == 0 {
		r.t.Fatalf("row %q has no action", row.Title)
	}
	row.Actions[0].Run(r.host)
}

func (r *formRig) pressTitled(title string) {
	r.t.Helper()
	r.press(r.row(title))
}

func (r *formRig) subtitleOf(title string) string {
	r.t.Helper()
	return r.row(title).Subtitle
}

func (r *formRig) lastInvoke() api.Invocation {
	r.t.Helper()
	if len(r.invokes) == 0 {
		r.t.Fatal("the submit action did not run")
	}
	return r.invokes[len(r.invokes)-1]
}

func textField(id, label string) api.Field {
	return api.FieldOf(id, api.FieldText, label)
}

func TestFormRowsShowEachFieldWithItsValueAndSaveLast(t *testing.T) {
	name := textField("name", "Name")
	name.Value = api.TextValue("Ada")
	notify := api.FieldOf("notify", api.FieldCheckbox, "Notify")
	tier := api.FieldOf("tier", api.FieldSelect, "Tier")
	tier.Options = []api.Option{{Title: "Free", Value: "free"}, {Title: "Pro", Value: "pro"}}
	tier.Value = api.TextValue("pro")
	note := api.FieldOf("note", api.FieldDescription, "Read me first")
	email := api.FieldOf("email", api.FieldEmail, "Email")
	r := openForm(t, api.FormSpec{Fields: []api.Field{name, notify, tier, note, email}, Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}, SubmitTitle: "Create"})

	rows := r.rows("")
	titles := rowTitles(rows)
	want := []string{"Name", "Notify", "Tier", "Read me first", "Email", "Create"}
	if len(titles) != len(want) {
		t.Fatalf("rows = %v; want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("rows = %v; want %v", titles, want)
		}
	}
	if got := findRow(t, rows, "Name").Subtitle; got != "Ada" {
		t.Fatalf("name subtitle = %q; want its value", got)
	}
	if got := findRow(t, rows, "Tier").Subtitle; got != "Pro" {
		t.Fatalf("tier subtitle = %q; want the option title", got)
	}
	if got := findRow(t, rows, "Notify").Accessory; got != "Off" {
		t.Fatalf("notify accessory = %q; want Off", got)
	}
	if !findRow(t, rows, "Read me first").Disabled {
		t.Fatal("a description row is selectable")
	}
}

func TestTextFieldIsEditedInTheSearchFieldAndCommitted(t *testing.T) {
	name := textField("name", "Name")
	name.Value = api.TextValue("Ada")
	r := openForm(t, api.FormSpec{Fields: []api.Field{name}})

	r.pressTitled("Name")
	if r.host.query != "Ada" {
		t.Fatalf("the search field holds %q; want the current value to edit", r.host.query)
	}
	if r.page.Placeholder != "Enter Name" {
		t.Fatalf("placeholder = %q; want the field's hint while editing", r.page.Placeholder)
	}
	r.host.query = "Grace"
	r.press(findRow(t, r.page.Search("Grace"), "Set Name"))

	if r.host.query != "" || r.page.Placeholder != formPlaceholder {
		t.Fatalf("after commit the query is %q and the placeholder %q; want both reset", r.host.query, r.page.Placeholder)
	}
	r.rows("")
	if r.page.SelectID != rowID("name") {
		t.Fatalf("select id = %q; want the edited row selected again", r.page.SelectID)
	}
	if got := r.subtitleOf("Name"); got != "Grace" {
		t.Fatalf("name subtitle = %q; want the committed value", got)
	}
}

func TestRequiredFieldBlocksSaveAndMarksItsRow(t *testing.T) {
	name := textField("name", "Name")
	name.Required = true
	r := openForm(t, api.FormSpec{Fields: []api.Field{name}, Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})

	r.pressTitled("Save")
	if len(r.invokes) != 0 {
		t.Fatal("a form with a missing required field was submitted")
	}
	r.rows("")
	if r.page.SelectID != rowID("name") {
		t.Fatalf("select id = %q; want the first invalid row selected", r.page.SelectID)
	}
	row := r.row("Name")
	if row.Subtitle != "Name is required" || row.Tone != "red" {
		t.Fatalf("name row = %q (%s); want the required message in red", row.Subtitle, row.Tone)
	}
}

func TestEmailFieldReportsTheValidatorCopy(t *testing.T) {
	email := api.FieldOf("email", api.FieldEmail, "Email")
	email.Value = api.TextValue("not-an-address")
	r := openForm(t, api.FormSpec{Fields: []api.Field{email}, Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})

	r.pressTitled("Save")
	if got := r.subtitleOf("Email"); got != "Enter an email address" {
		t.Fatalf("email subtitle = %q; want the validator's copy", got)
	}
}

func TestCheckboxFlipsWithoutEditing(t *testing.T) {
	notify := api.FieldOf("notify", api.FieldCheckbox, "Notify")
	r := openForm(t, api.FormSpec{Fields: []api.Field{notify}})

	r.pressTitled("Notify")
	if r.page.Placeholder != formPlaceholder {
		t.Fatal("a checkbox entered the editing state")
	}
	if got := r.row("Notify").Accessory; got != "On" {
		t.Fatalf("accessory = %q; want On after one press", got)
	}
	if r.host.refreshes != 1 {
		t.Fatalf("refreshes = %d; want the row to redraw once", r.host.refreshes)
	}
}

func TestSelectFieldFiltersOptionsAndCommitsOne(t *testing.T) {
	tier := api.FieldOf("tier", api.FieldSelect, "Tier")
	tier.Options = []api.Option{{Title: "Free", Value: "free"}, {Title: "Pro", Value: "pro"}}
	r := openForm(t, api.FormSpec{Fields: []api.Field{tier}})

	r.pressTitled("Tier")
	if got := rowTitles(r.rows("")); len(got) != 2 {
		t.Fatalf("options = %v; want both", got)
	}
	if got := rowTitles(r.rows("fr")); len(got) != 1 || got[0] != "Free" {
		t.Fatalf("filtered options = %v; want Free only", got)
	}
	r.press(findRow(t, r.rows("fr"), "Free"))
	if got := r.subtitleOf("Tier"); got != "Free" {
		t.Fatalf("tier subtitle = %q; want the chosen option", got)
	}
}

func TestMultiselectTogglesOptionsUntilDone(t *testing.T) {
	tags := api.FieldOf("tags", api.FieldMultiselect, "Tags")
	tags.Options = []api.Option{{Title: "Red", Value: "red"}, {Title: "Blue", Value: "blue"}}
	r := openForm(t, api.FormSpec{Fields: []api.Field{tags}})

	r.pressTitled("Tags")
	if rowTitles(r.rows(""))[0] != "Done" {
		t.Fatal("a multiselect does not offer Done first")
	}
	r.press(findRow(t, r.rows(""), "Red"))
	r.press(findRow(t, r.rows(""), "Blue"))
	r.press(findRow(t, r.rows(""), "Red"))
	r.press(findRow(t, r.rows(""), "Done"))
	if got := r.subtitleOf("Tags"); got != "1 selected" {
		t.Fatalf("tags subtitle = %q; want one selection after toggling Red twice", got)
	}
	if r.page.Placeholder != formPlaceholder {
		t.Fatal("Done left the field in the editing state")
	}
}

func TestSubmitRunsTheSubmitActionWithEveryValue(t *testing.T) {
	name := textField("name", "Name")
	name.Value = api.TextValue("Ada")
	notify := api.FieldOf("notify", api.FieldCheckbox, "Notify")
	note := api.FieldOf("note", api.FieldDescription, "Context only")
	r := openForm(t, api.FormSpec{Fields: []api.Field{name, notify, note}, Submit: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})

	r.pressTitled("Save")
	in := r.lastInvoke()
	if in.Ref.ID != "create" {
		t.Fatalf("submit ran %q; want create", in.Ref.ID)
	}
	if in.Form["name"].Text != "Ada" || in.Form["notify"].Kind != api.FormFlag || in.Form["notify"].Flag {
		t.Fatalf("form = %+v; want the text and the unchecked flag", in.Form)
	}
	if _, ok := in.Form["note"]; ok {
		t.Fatal("a description row was sent as a value")
	}
	if !r.host.hidden {
		t.Fatal("a submit with an empty result did not close the palette")
	}
}

func TestSubmitWithoutAnActionSaysSo(t *testing.T) {
	name := textField("name", "Name")
	r := openForm(t, api.FormSpec{Fields: []api.Field{name}})
	r.pressTitled("Save")
	if len(r.invokes) != 0 || len(r.host.toasts) != 1 {
		t.Fatalf("invokes = %d, toasts = %v; want one toast and no run", len(r.invokes), r.host.toasts)
	}
}

func TestPasswordIsNeverEchoed(t *testing.T) {
	secret := api.FieldOf("secret", api.FieldPassword, "Token")
	secret.Value = api.TextValue("hunter2")
	r := openForm(t, api.FormSpec{Fields: []api.Field{secret}})

	if got := r.subtitleOf("Token"); got != concealedValue {
		t.Fatalf("password subtitle = %q; want the concealed marker", got)
	}
	r.pressTitled("Token")
	if r.host.query != "" {
		t.Fatalf("editing a password prefilled the search field with %q", r.host.query)
	}
	set := r.rows("hunter3")
	for _, row := range set {
		if strings.Contains(row.Title, "hunter") || strings.Contains(row.Subtitle, "hunter") {
			t.Fatalf("a row shows the password: %+v", row)
		}
	}
}

func TestMultilineFieldOpensAnEditorThatReturnsItsText(t *testing.T) {
	bio := api.FieldOf("bio", api.FieldTextarea, "Bio")
	bio.Value = api.TextValue("line one")
	r := openForm(t, api.FormSpec{Fields: []api.Field{bio}})

	r.pressTitled("Bio")
	if len(r.host.pushed) != 1 || r.host.pushed[0].Title != "Bio" {
		t.Fatalf("pushed = %d views; want one editor titled Bio", len(r.host.pushed))
	}
	if editor := r.session.form.editor; editor == nil || editor.text != "line one" {
		t.Fatalf("editor = %+v; want the field's text", editor)
	}
}

func TestMultilineSaveHandsTheTextBackToTheField(t *testing.T) {
	bio := api.FieldOf("bio", api.FieldTextarea, "Bio")
	bio.Value = api.TextValue("old")
	r := openForm(t, api.FormSpec{Fields: []api.Field{bio}})
	r.pressTitled("Bio")

	r.session.form.editor.submitFn(r.host, "first line\nsecond line")
	if r.host.pops != 1 {
		t.Fatalf("pops = %d; want the editor closed on save", r.host.pops)
	}
	if got := r.subtitleOf("Bio"); got != "2 lines" {
		t.Fatalf("bio subtitle = %q; want the saved text", got)
	}
}
