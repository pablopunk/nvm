// Example: a command that opens a structured form. Values are validated with the same rules the
// host applies, and the submit handler saves through Settings and closes the form.
package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "settings", Title: "Quick Note Settings", Icon: "sticky-note", Aliases: []string{"qn"}}}, nil
}

func noteFields() []api.Field {
	maxLength := 40
	name := api.FieldOf("folder-name", api.FieldText, "Folder name")
	name.Required = true
	name.Rules = &api.Rules{MaxLength: maxLength}
	folder := api.FieldOf("folder", api.FieldFolder, "Save notes in")
	folder.DefaultPath = "~/Documents"
	format := api.FieldOf("format", api.FieldSelect, "Format")
	format.Options = []api.Option{{Title: "Markdown", Value: "markdown"}, {Title: "Plain text", Value: "text"}}
	format.Value = api.TextValue("markdown")
	tags := api.FieldOf("tags", api.FieldMultiselect, "Default tags")
	tags.Options = []api.Option{{Title: "Work", Value: "work"}, {Title: "Personal", Value: "personal"}}
	notify := api.FieldOf("notify", api.FieldCheckbox, "Show a notification on save")
	notify.Value = api.FlagValue(true)
	return []api.Field{name, folder, format, tags, notify}
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "settings":
		submit := api.Run("save", "Save", nil)
		view := api.FormView("Quick Note Settings", noteFields(), submit)
		return api.Result{View: &view, Navigation: api.NavPush}, nil
	case "save":
		return saveSettings(ctx, in)
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

func saveSettings(ctx api.Context, in api.Invocation) (api.Result, error) {
	for _, field := range noteFields() {
		if err := api.ValidateFieldValue(field, in.Form[field.ID]); err != nil {
			return api.Result{}, fmt.Errorf("%s: %w", field.Label, err)
		}
	}
	if err := ctx.Storage().Set("defaults", in.Form); err != nil {
		return api.Result{}, err
	}
	return ctx.Navigation().Pop(), nil
}
