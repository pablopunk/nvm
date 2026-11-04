package api

// Pure constructors for the common declarative shapes. They never touch the host: they build the
// same values an author could write as literals, so tests can compare them directly. Each returns
// a value; nothing runs until the host performs the result.

// Run builds an ActionRun that calls the extension's RunAction with ref id and string arguments.
func Run(id, title string, args map[string]string) Action {
	return Action{Kind: ActionRun, Title: title, Ref: &ActionRef{ID: id, Args: args}}
}

// OpenURL opens an http, https or mailto URL in the default handler.
func OpenURL(title, url string) Action {
	return Action{Kind: ActionOpenURL, Title: title, URL: url}
}

// OpenPath opens a local file or folder with its default app.
func OpenPath(title, path string) Action {
	return Action{Kind: ActionOpenPath, Title: title, Path: path}
}

// RevealPath shows a path in the OS file manager.
func RevealPath(title, path string) Action {
	return Action{Kind: ActionRevealPath, Title: title, Path: path}
}

// CopyText writes text to the clipboard. Concealed keeps the write out of clipboard history.
func CopyText(title, text string, concealed bool) Action {
	return Action{Kind: ActionCopyText, Title: title, Text: text, Paste: &PasteOptions{Concealed: concealed}}
}

// PasteText pastes text into the frontmost app. The host does not restore the clipboard afterwards, so the text stays on it.
func PasteText(title, text string) Action {
	return Action{Kind: ActionPasteText, Title: title, Text: text}
}

// SetQuery replaces the palette's search text and searches again, keeping the palette open.
func SetQuery(title, query string) Action {
	return Action{Kind: ActionSetQuery, Title: title, Query: query}
}

// SystemOp runs one OS command such as "lock-screen" or "empty-trash" on the host. Destructive ones ask first when
// the action carries a Confirmation.
func SystemOp(title, op string) Action {
	return Action{Kind: ActionSystem, Title: title, Op: op}
}

// PushView drills into a child view; Escape returns to the parent.
func PushView(title string, view View) Action {
	return Action{Kind: ActionPushView, Title: title, View: &view}
}

// RootView replaces the navigation stack with a top-level destination.
func RootView(title string, view View) Action {
	return Action{Kind: ActionRootView, Title: title, View: &view}
}

// WithConfirmation asks before the action runs, as a palette row.
func WithConfirmation(action Action, confirmation Confirmation) Action {
	action.Confirm = &confirmation
	return action
}

// Destructive marks an action as destructive without adding a confirmation.
func Destructive(action Action) Action {
	action.Style = "destructive"
	return action
}

// Primary is a convenience for the Primary field of an item.
func Primary(action Action) *Action {
	return &action
}

// ListView builds a list view. Items must have stable ids.
func ListView(title string, items ...Item) View {
	return View{Kind: ViewList, Title: title, Items: items}
}

// GridView builds a grid view of tiles.
func GridView(title string, columns int, items ...Item) View {
	return View{Kind: ViewGrid, Title: title, Items: items, Grid: &GridSpec{Columns: columns}}
}

// FormView builds a structured form whose submit action receives the field values.
func FormView(title string, fields []Field, submit Action) View {
	return View{Kind: ViewForm, Title: title, Form: &FormSpec{Fields: fields, Submit: &submit}}
}

// EditorView builds an editable text or Markdown surface.
func EditorView(title, content string, submit Action) View {
	return View{Kind: ViewEditor, Title: title, Editor: &EditorSpec{Content: content, Format: "markdown", Submit: &submit}}
}

// PreviewView builds a read-only Markdown preview.
func PreviewView(title, markdown string) View {
	return View{Kind: ViewPreview, Title: title, Preview: &PreviewSpec{Markdown: markdown}}
}

// ProgressView builds a step progress view.
func ProgressView(title string, steps ...ProgressStep) View {
	return View{Kind: ViewProgress, Title: title, Progress: &ProgressSpec{Steps: steps}}
}

// CollectionView builds a host-owned CRUD list.
func CollectionView(title string, add *Action, records ...CrudItem) View {
	return View{Kind: ViewCollection, Title: title, Collection: &CollectionSpec{Add: add, Items: records}}
}

// FieldOf builds a field with a type and a label; callers add Required, Options or Rules.
func FieldOf(id string, fieldType FieldType, label string) Field {
	return Field{ID: id, Type: fieldType, Label: label}
}

// Section groups actions for an action panel.
func Section(title string, actions ...Action) ActionSection {
	return ActionSection{Title: title, Actions: actions}
}

// Panel builds an action panel from sections.
func Panel(sections ...ActionSection) *ActionPanel {
	return &ActionPanel{Sections: sections}
}

// ErrorView is the failure or invalid-input view with a title and the corrective message the user must read. The
// message is one contextual item, which the palette draws at full contrast and wraps.
func ErrorView(title, message string) View {
	return ListView(title, Item{ID: "error", Title: title, Subtitle: message, Disabled: true, Contextual: true})
}
