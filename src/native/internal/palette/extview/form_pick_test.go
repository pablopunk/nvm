package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestPathFieldOpensTheNativeDialogAndStoresThePick(t *testing.T) {
	avatar := api.FieldOf("avatar", api.FieldFile, "Avatar")
	avatar.Extensions = []string{".png", "jpg"}
	avatar.FilterName = "Images"
	avatar.ButtonLabel = "Pick image"
	r := openForm(t, api.FormSpec{Fields: []api.Field{avatar}})
	r.dialog.Answer("/pictures/ada.png")

	r.pressTitled("Avatar")
	r.ui.settle(t)

	request := r.dialog.Requests()
	if len(request) != 1 || request[0].Folder || request[0].Multiple {
		t.Fatalf("dialog requests = %+v; want one file dialog", request)
	}
	if got := request[0].Extensions; len(got) != 2 || got[0] != "png" || got[1] != "jpg" {
		t.Fatalf("extensions = %v; want the suffixes without dots", got)
	}
	if request[0].FilterName != "Images" || request[0].ButtonLabel != "Pick image" {
		t.Fatalf("request = %+v; want the field's filter and button", request[0])
	}
	if got := r.subtitleOf("Avatar"); got != "/pictures/ada.png" {
		t.Fatalf("avatar subtitle = %q; want the picked path", got)
	}
}

func TestCanceledPickerChangesNothing(t *testing.T) {
	avatar := api.FieldOf("avatar", api.FieldFile, "Avatar")
	r := openForm(t, api.FormSpec{Fields: []api.Field{avatar}})

	r.pressTitled("Avatar")
	r.ui.settle(t)
	if got := r.subtitleOf("Avatar"); got != "Not set" {
		t.Fatalf("avatar subtitle = %q; want it unchanged after a cancel", got)
	}
}

func TestFolderAndFilesFieldsUseTheirOwnDialogModes(t *testing.T) {
	folder := api.FieldOf("folder", api.FieldFolder, "Backups")
	files := api.FieldOf("files", api.FieldFiles, "Attachments")
	r := openForm(t, api.FormSpec{Fields: []api.Field{folder, files}})
	r.dialog.Answer("/backups")
	r.dialog.Answer("/a.txt", "/b.txt")

	r.pressTitled("Backups")
	r.ui.settle(t)
	r.pressTitled("Attachments")
	r.ui.settle(t)

	requests := r.dialog.Requests()
	if !requests[0].Folder || requests[0].CreateFolders != true {
		t.Fatalf("folder request = %+v; want a folder dialog that can create folders", requests[0])
	}
	if !requests[1].Multiple {
		t.Fatalf("files request = %+v; want a multiple-selection dialog", requests[1])
	}
	if got := r.subtitleOf("Attachments"); got != "2 selected" {
		t.Fatalf("attachments subtitle = %q; want every pick kept", got)
	}
}

func TestPickerWithoutADialogSaysSo(t *testing.T) {
	avatar := api.FieldOf("avatar", api.FieldFile, "Avatar")
	session := NewSession(Deps{Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) {}}, "acme.form",
		api.View{Kind: api.ViewForm, Title: "Create", Form: &api.FormSpec{Fields: []api.Field{avatar}}})
	host := &pageHost{}
	page := session.Palette()
	findRow(t, page.Search(""), "Avatar").Actions[0].Run(host)
	if len(host.toasts) != 1 {
		t.Fatalf("toasts = %v; want the unavailable notice", host.toasts)
	}
}
