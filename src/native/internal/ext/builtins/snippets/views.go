package snippets

import "github.com/pablopunk/nvm/native/internal/ext/api"

// The form's fields, by id.
const (
	fieldName    = "name"
	fieldKeyword = "keyword"
	fieldContent = "content"
)

// Placeholder and empty-list copy the native Snippets list kept.
const (
	searchPlaceholder = "Search Snippets"
	emptyTitle        = "No snippets yet."
	emptySubtitle     = "Create one to paste text quickly."
)

// listView is the Snippets list: the create row, then the snippets by name.
func listView(all []Snippet, canPaste bool, refresh *api.Refresh) api.View {
	view := api.ListView(listTitle, listItems(all, canPaste)...)
	view.Placeholder = searchPlaceholder
	view.EmptyView = &api.EmptyState{Title: emptyTitle, Subtitle: emptySubtitle}
	view.Refresh = refresh
	return view
}

// formView is the form that creates a snippet, or edits the given one. The name and the content are required; the
// keyword is optional and unique.
func formView(title string, sn Snippet, submit api.Action) api.View {
	fields := []api.Field{
		requiredField(fieldName, "Name", "Email signature", sn.Name),
		optionalField(fieldKeyword, "Keyword (optional, type it in the palette to find the snippet)", ";sig", sn.Keyword),
		contentField(sn.Text),
	}
	return api.FormView(title, fields, submit)
}

// requiredField is a single-line field the form cannot save without.
func requiredField(id, label, placeholder, value string) api.Field {
	field := textField(id, label, placeholder, value)
	field.Required = true
	return field
}

// optionalField is a single-line field the form can save without.
func optionalField(id, label, placeholder, value string) api.Field {
	return textField(id, label, placeholder, value)
}

// textField is a single-line text field with its placeholder and value.
func textField(id, label, placeholder, value string) api.Field {
	field := api.FieldOf(id, api.FieldText, label)
	field.Placeholder = placeholder
	field.Value = api.TextValue(value)
	return field
}

// contentField is the multi-line text the snippet pastes.
func contentField(text string) api.Field {
	field := api.FieldOf(fieldContent, api.FieldTextarea, "Content")
	field.Placeholder = "Text to paste…"
	field.Required = true
	field.Value = api.TextValue(text)
	return field
}
