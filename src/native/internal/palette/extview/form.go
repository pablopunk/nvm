package extview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// formPlaceholder is the search hint while no field is being edited.
const formPlaceholder = "Filter fields"

// formState is the palette form of one ViewForm. It holds the values until the submit action runs, the validation
// errors, and the field being edited. A field is edited in the search field, as the Electron form does: its text is
// the query, and Enter commits it.
type formState struct {
	sess     *Session
	spec     api.FormSpec
	values   api.FormValues
	errors   map[string]string
	editing  string
	selectID string
	page     *palette.View
	// editor is the multi-line editor open for a textarea field, kept so its save reaches the field.
	editor *editorState
}

// formPage returns the session's form page, creating its state on first use.
func (s *Session) formPage() *palette.View {
	if s.form == nil {
		s.form = newFormState(s)
	}
	return s.form.page
}

func newFormState(s *Session) *formState {
	f := &formState{sess: s, spec: *s.view.Form, values: api.FormValues{}, errors: map[string]string{}}
	for _, field := range f.spec.Fields {
		if takesValue(field.Type) {
			f.values[field.ID] = defaultFieldValue(field)
		}
	}
	f.page = &palette.View{
		Title: s.view.Title, Placeholder: formPlaceholder, Hint: "↵ Edit   " + saveHint(),
		Search: f.rows,
		OnKey:  f.key,
	}
	return f
}

// rows answers the search: the fields (filtered by the query) and the save row, or the choices of the field being edited.
func (f *formState) rows(q string) []search.Item {
	f.page.SelectID = f.takeSelectID()
	if f.editing != "" {
		return f.editingRows(f.fieldByID(f.editing), q)
	}
	var out []search.Item
	for _, field := range f.spec.Fields {
		if takesValue(field.Type) && !matchesFilter(field, f.value(field), q) {
			continue
		}
		out = append(out, f.fieldRow(field))
	}
	return append(out, f.saveRow())
}

func (f *formState) takeSelectID() string {
	id := f.selectID
	f.selectID = ""
	return id
}

func (f *formState) fieldRow(field api.Field) search.Item {
	if !takesValue(field.Type) {
		return contextRow(field)
	}
	row := search.Item{
		ID: rowID(field.ID), Title: fieldName(field), Subtitle: fieldValueLabel(field, f.value(field)),
		Icon: fieldIcon(field.Type), Disabled: field.Disabled,
		Actions: []search.Action{{Title: "Edit", Dismiss: search.DismissNever, Run: func(h search.Host) { f.open(h, field) }}},
	}
	if field.Type == api.FieldCheckbox {
		row.Accessory = fieldValueLabel(field, f.value(field))
	}
	if message := f.errors[field.ID]; message != "" {
		row.Subtitle, row.Tone = message, "red"
	}
	return row
}

// contextRow is a description or separator: it stays visible as context and takes no input.
func contextRow(field api.Field) search.Item {
	title, subtitle := field.Label, field.Description
	if title == "" {
		title, subtitle = field.Description, ""
	}
	return search.Item{ID: rowID(field.ID), Title: title, Subtitle: subtitle, Disabled: true}
}

func rowID(fieldID string) string {
	return "form:field:" + fieldID
}

func (f *formState) saveRow() search.Item {
	return search.Item{
		ID: "form:save", Title: f.saveTitle(), Icon: "check",
		Actions: []search.Action{{Title: f.saveTitle(), Dismiss: search.DismissNever, Run: f.save}},
	}
}

func (f *formState) saveTitle() string {
	if f.spec.SubmitTitle != "" {
		return f.spec.SubmitTitle
	}
	return "Save"
}

// key handles Escape while a field is edited (it stops editing and keeps the form open) and Command-Enter (save).
func (f *formState) key(c *ui.Context, h search.Host) bool {
	switch {
	case f.editing != "" && c.Shortcut(0, ui.KeyEscape):
		f.stopEditing(h)
	case c.Shortcut(palette.Mod, ui.KeyEnter):
		f.save(h)
	default:
		return false
	}
	return true
}

// open starts the interaction of a field row: a checkbox flips, a path with no value opens the picker, a multi-line
// field opens an editor, and every other field is edited in the search field.
func (f *formState) open(h search.Host, field api.Field) {
	if field.Disabled {
		return
	}
	value := f.value(field)
	switch {
	case field.Type == api.FieldCheckbox:
		f.setValue(field.ID, api.FlagValue(!value.Flag))
		h.Refresh()
	case isPath(field.Type) && isEmptyValue(value):
		f.choose(h, field)
	case field.Type == api.FieldTextarea:
		f.editor = f.sess.multilineEditor(field, value.Text, func(h search.Host, text string) {
			f.commit(h, field, api.TextValue(text))
		})
		h.Push(f.editor.page)
	default:
		f.editing = field.ID
		f.page.Placeholder = fieldPlaceholder(field)
		f.page.Concealed = field.Type == api.FieldPassword
		h.SetQuery(prefillFor(field, value))
	}
}

// editingRows are the answers of the field being edited: options for a choice, a choose and clear pair for a path,
// and one row that sets the typed text for every other kind.
func (f *formState) editingRows(field api.Field, q string) []search.Item {
	switch {
	case isChoice(field.Type):
		return f.choiceRows(field, q)
	case isPath(field.Type):
		return f.pathRows(field, q)
	}
	return []search.Item{f.textRow(field, q)}
}

func (f *formState) textRow(field api.Field, q string) search.Item {
	value := q
	if field.Type != api.FieldPassword {
		value = strings.TrimSpace(q)
	}
	title := "Set " + fieldName(field)
	if value == "" {
		title = "Clear " + fieldName(field)
	}
	subtitle := value
	if field.Type == api.FieldPassword {
		subtitle = ""
	}
	return search.Item{
		ID: "form:apply:" + field.ID, Title: title, Subtitle: subtitle, Icon: "check",
		Actions: []search.Action{{Title: "Set", Dismiss: search.DismissNever, Run: func(h search.Host) {
			f.commit(h, field, api.TextValue(value))
		}}},
	}
}

func (f *formState) choiceRows(field api.Field, q string) []search.Item {
	var rows []search.Item
	if field.Type == api.FieldMultiselect {
		rows = append(rows, search.Item{ID: "form:done", Title: "Done", Icon: "chevron-right",
			Actions: []search.Action{{Title: "Done", Dismiss: search.DismissNever, Run: func(h search.Host) { f.stopEditing(h) }}}})
	}
	selected := f.value(field)
	for _, option := range field.Options {
		if !matchesOption(option, q) {
			continue
		}
		rows = append(rows, f.optionRow(field, option, selected))
	}
	return rows
}

func (f *formState) optionRow(field api.Field, option api.Option, selected api.FormValue) search.Item {
	on := option.Value == selected.Text
	if field.Type == api.FieldMultiselect {
		on = containsString(selected.List, option.Value)
	}
	row := search.Item{ID: "form:option:" + option.Value, Title: option.Title, Icon: "check", Accessory: onOff(on)}
	row.Actions = []search.Action{{Title: option.Title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		if field.Type == api.FieldMultiselect {
			f.toggleOption(h, field, option.Value)
			return
		}
		f.commit(h, field, api.TextValue(option.Value))
	}}}
	return row
}

func (f *formState) toggleOption(h search.Host, field api.Field, value string) {
	current := f.value(field).List
	next := make([]string, 0, len(current)+1)
	if containsString(current, value) {
		for _, item := range current {
			if item != value {
				next = append(next, item)
			}
		}
	} else {
		next = append(append(next, current...), value)
	}
	f.setValue(field.ID, api.ListValue(next...))
	h.Refresh()
}

func (f *formState) pathRows(field api.Field, q string) []search.Item {
	choose := search.Item{ID: "form:choose-path", Title: chooseLabel(field), Icon: "folder",
		Actions: []search.Action{{Title: chooseLabel(field), Dismiss: search.DismissNever, Run: func(h search.Host) { f.choose(h, field) }}}}
	clear := search.Item{ID: "form:clear-path", Title: "Clear " + fieldName(field), Icon: "x",
		Actions: []search.Action{{Title: "Clear", Dismiss: search.DismissNever, Run: func(h search.Host) {
			f.commit(h, field, emptyValueFor(field))
		}}}}
	var rows []search.Item
	for _, row := range []search.Item{choose, clear} {
		if matchesText(row.Title, q) {
			rows = append(rows, row)
		}
	}
	return rows
}

func chooseLabel(field api.Field) string {
	if field.ButtonLabel != "" {
		return field.ButtonLabel
	}
	return "Choose " + fieldName(field)
}

// commit stores a value, leaves the editing state and selects the field's row again.
func (f *formState) commit(h search.Host, field api.Field, value api.FormValue) {
	f.setValue(field.ID, value)
	f.selectID = rowID(field.ID)
	f.stopEditingState()
	h.SetQuery("")
}

// stopEditing leaves the editing state without a value change; the query is cleared and the field's row selected.
func (f *formState) stopEditing(h search.Host) {
	f.selectID = rowID(f.editing)
	f.stopEditingState()
	h.SetQuery("")
}

func (f *formState) stopEditingState() {
	f.editing = ""
	f.page.Placeholder = formPlaceholder
	f.page.Concealed = false
}

// save validates every field the way the host does before a submit runs. A failure marks its rows and selects the
// first; success runs the submit action with every field's value.
func (f *formState) save(h search.Host) {
	f.errors = map[string]string{}
	first := ""
	for _, field := range f.spec.Fields {
		if !takesValue(field.Type) || field.Disabled {
			continue
		}
		if err := api.ValidateFieldValue(field, f.value(field)); err != nil {
			f.errors[field.ID] = fieldErrorCopy(field, err)
			if first == "" {
				first = field.ID
			}
		}
	}
	if first != "" {
		f.selectID = rowID(first)
		h.Refresh()
		return
	}
	if f.spec.Submit == nil || f.spec.Submit.Ref == nil {
		h.Toast(unavailableMessage)
		return
	}
	f.sess.submit(h, api.Invocation{Ref: *f.spec.Submit.Ref, Form: f.submittedValues()}, nil)
}

// submittedValues returns the values of the fields that take one, the same set the host validated.
func (f *formState) submittedValues() api.FormValues {
	out := api.FormValues{}
	for _, field := range f.spec.Fields {
		if takesValue(field.Type) {
			out[field.ID] = f.value(field)
		}
	}
	return out
}

func (f *formState) value(field api.Field) api.FormValue {
	if value, ok := f.values[field.ID]; ok {
		return value
	}
	return defaultFieldValue(field)
}

func (f *formState) setValue(id string, value api.FormValue) {
	f.values[id] = value
	delete(f.errors, id)
}

func (f *formState) fieldByID(id string) api.Field {
	for _, field := range f.spec.Fields {
		if field.ID == id {
			return field
		}
	}
	return api.Field{ID: id}
}
