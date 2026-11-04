package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.form is the dev fixture of the form view: one field of each of the sixteen types, the declarative rules the
// host checks, and a submit that answers with what it received. The prompt command opens two of the same fields as a
// palette prompt, which submits through the same path.

// SearchItems offers the fixture's view as a root row for queries that name it, so the palette can open it.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !strings.Contains(strings.ToLower("Form fixture: create a ticket"), strings.ToLower(query)) {
		return nil, nil
	}
	return []api.Item{{ID: "form", Title: "Form fixture: create a ticket", Primary: api.Primary(api.Run("form", "Open", nil))}}, nil
}

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "form", Title: "Form fixture"},
		{ID: "prompt", Title: "Prompt fixture"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "form":
		view := api.FormView("Create Ticket", allFields(), api.Run("save", "Create", nil))
		view.Form.SubmitTitle = "Create"
		return api.Result{View: &view}, nil
	case "prompt":
		prompt := ctx.Input().Prompt(api.PromptSpec{
			Title:       "Name the ticket",
			Message:     "Two short answers are enough for a prompt.",
			Fields:      []api.Field{promptName(), promptPriority()},
			Action:      api.Run("save", "Create", nil),
			SubmitTitle: "Create",
		})
		view := api.ListView("Prompt fixture", api.Item{ID: "ask", Title: "Ask for a name", Primary: api.Primary(prompt)})
		return api.Result{View: &view}, nil
	case "save":
		view := summaryView(in.Form)
		return api.Result{View: &view}, nil
	}
	return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
}

func allFields() []api.Field {
	name := api.FieldOf("name", api.FieldText, "Name")
	name.Required = true
	name.Rules = &api.Rules{MinLength: 2, MaxLength: 40}
	bio := api.FieldOf("bio", api.FieldTextarea, "Bio")
	bio.Rows = 3
	token := api.FieldOf("token", api.FieldPassword, "API token")
	email := api.FieldOf("email", api.FieldEmail, "Email")
	site := api.FieldOf("site", api.FieldURL, "Website")
	seats := api.FieldOf("seats", api.FieldNumber, "Seats")
	minimum, maximum := 1.0, 50.0
	seats.Rules = &api.Rules{Min: &minimum, Max: &maximum}
	due := api.FieldOf("due", api.FieldDate, "Due date")
	terms := api.FieldOf("terms", api.FieldCheckbox, "Accept the terms")
	terms.Required = true
	plan := api.FieldOf("plan", api.FieldDropdown, "Plan")
	plan.Options = []api.Option{{Title: "Free", Value: "free"}, {Title: "Pro", Value: "pro"}}
	tier := api.FieldOf("tier", api.FieldSelect, "Tier")
	tier.Options = []api.Option{{Title: "Starter", Value: "starter"}, {Title: "Team", Value: "team"}}
	tags := api.FieldOf("tags", api.FieldMultiselect, "Tags")
	tags.Options = []api.Option{{Title: "Bug", Value: "bug"}, {Title: "Idea", Value: "idea"}, {Title: "Chore", Value: "chore"}}
	avatar := api.FieldOf("avatar", api.FieldFile, "Avatar")
	avatar.Extensions = []string{"png", "jpg"}
	avatar.FilterName = "Images"
	attachments := api.FieldOf("attachments", api.FieldFiles, "Attachments")
	backups := api.FieldOf("backups", api.FieldFolder, "Backups")
	note := api.FieldOf("note", api.FieldDescription, "Fill in what you can; only the name and terms are required.")
	separator := api.FieldOf("separator", api.FieldSeparator, "")
	return []api.Field{name, bio, token, email, site, seats, due, terms, plan, tier, tags, avatar, attachments, backups, note, separator}
}

func promptName() api.Field {
	name := api.FieldOf("name", api.FieldText, "Name")
	name.Required = true
	return name
}

func promptPriority() api.Field {
	priority := api.FieldOf("priority", api.FieldSelect, "Priority")
	priority.Options = []api.Option{{Title: "Low", Value: "low"}, {Title: "High", Value: "high"}}
	return priority
}

// summaryView lists every submitted value as a row. A password is reported as set or not, never as its text.
func summaryView(form api.FormValues) api.View {
	ids := make([]string, 0, len(form))
	for id := range form {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := make([]api.Item, 0, len(ids))
	for _, id := range ids {
		title := fmt.Sprintf("%s: %s", id, describe(id, form[id]))
		items = append(items, api.Item{ID: id, Title: title})
	}
	return api.ListView("Submitted", items...)
}

func describe(id string, value api.FormValue) string {
	switch {
	case id == "token":
		if value.Text != "" {
			return "set"
		}
		return "not set"
	case value.Kind == api.FormFlag:
		return fmt.Sprint(value.Flag)
	case value.Kind == api.FormList:
		return strings.Join(value.List, ", ")
	}
	return value.Text
}
