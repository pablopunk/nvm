package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.picker is the dev fixture of the native pickers: a file, a set of files and a folder, each opening the
// operating system's dialog from its row. Submit answers with the paths it received.

// SearchItems offers the fixture's view as a root row for queries that name it, so the palette can open it.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !strings.Contains(strings.ToLower("Picker fixture: pick files"), strings.ToLower(query)) {
		return nil, nil
	}
	return []api.Item{{ID: "pick", Title: "Picker fixture: pick files", Primary: api.Primary(api.Run("pick", "Open", nil))}}, nil
}

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "pick", Title: "Picker fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "pick":
		avatar := api.FieldOf("avatar", api.FieldFile, "Avatar")
		avatar.Extensions = []string{".png", ".jpg"}
		avatar.FilterName = "Images"
		avatar.ButtonLabel = "Choose image"
		attachments := api.FieldOf("attachments", api.FieldFiles, "Attachments")
		backups := api.FieldOf("backups", api.FieldFolder, "Backup folder")
		backups.DisableDirectoryCreation = true
		view := api.FormView("Pick Files", []api.Field{avatar, attachments, backups}, api.Run("save", "Attach", nil))
		return api.Result{View: &view}, nil
	case "save":
		view := api.ListView("Picked", pickedItems(in.Form)...)
		return api.Result{View: &view}, nil
	}
	return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
}

func pickedItems(form api.FormValues) []api.Item {
	return []api.Item{
		{ID: "avatar", Title: "avatar: " + form["avatar"].Text},
		{ID: "attachments", Title: "attachments: " + strings.Join(form["attachments"].List, ", ")},
		{ID: "backups", Title: "backups: " + form["backups"].Text},
	}
}
