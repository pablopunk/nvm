package quicklinks

import "github.com/pablopunk/nvm/native/internal/ext/api"

// The form's fields, by id.
const (
	fieldName    = "name"
	fieldKeyword = "keyword"
	fieldURL     = "url"
)

// createView is the Create Quicklink form: a name, a keyword and an address with {query} for the search text.
func createView() api.View {
	fields := []api.Field{
		requiredField(fieldName, "Name", "Hacker News", api.FieldText),
		requiredField(fieldKeyword, "Keyword", "hn", api.FieldText),
		requiredField(fieldURL, "URL (use {query} for the search text)", "https://hn.algolia.com/?q={query}", api.FieldURL),
	}
	return api.FormView(CreateTitle, fields, api.Run(saveActionID, "Save", nil))
}

// requiredField is a field the form cannot save without.
func requiredField(id, label, placeholder string, fieldType api.FieldType) api.Field {
	field := api.FieldOf(id, fieldType, label)
	field.Placeholder = placeholder
	field.Required = true
	return field
}
