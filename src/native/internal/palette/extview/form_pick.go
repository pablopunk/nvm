package extview

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// choose opens the operating system's dialog for a path field. The dialog blocks, so it runs off the UI thread and the
// chosen paths come back on it; a canceled dialog changes nothing.
func (f *formState) choose(h search.Host, field api.Field) {
	dialog := f.sess.deps.Dialog
	if dialog == nil {
		h.Toast(unavailableMessage)
		return
	}
	request := chooseRequest(field)
	runOffUI(f.sess, func() ([]string, error) {
		return dialog.Choose(request)
	}, func(paths []string, err error) {
		if err != nil {
			h.Toast("The file dialog could not open")
			return
		}
		if len(paths) == 0 {
			return
		}
		f.setValue(field.ID, pathValue(field, paths))
		if f.editing == field.ID {
			f.stopEditing(h)
		}
		h.Refresh()
	})
}

// chooseRequest is the dialog the field asks for: its title, button, filter and folder rules.
func chooseRequest(field api.Field) platform.ChooseRequest {
	return platform.ChooseRequest{
		Title:         fieldName(field),
		ButtonLabel:   field.ButtonLabel,
		DefaultPath:   field.DefaultPath,
		Extensions:    trimmedExtensions(field.Extensions),
		FilterName:    field.FilterName,
		Folder:        field.Type == api.FieldFolder,
		Multiple:      field.Type == api.FieldFiles,
		CreateFolders: !field.DisableDirectoryCreation,
	}
}

func trimmedExtensions(extensions []string) []string {
	out := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		if trimmed := strings.TrimPrefix(extension, "."); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// pathValue is the value a path field takes from the paths the dialog returned: a list for files, else the first path.
func pathValue(field api.Field, paths []string) api.FormValue {
	if field.Type == api.FieldFiles {
		return api.ListValue(paths...)
	}
	return api.TextValue(paths[0])
}

func emptyValueFor(field api.Field) api.FormValue {
	if isList(field.Type) {
		return api.ListValue()
	}
	return api.TextValue("")
}

func isEmptyValue(value api.FormValue) bool {
	return value.Text == "" && len(value.List) == 0
}

// runOffUI runs work on its own goroutine and passes its result to done on the UI thread.
func runOffUI[T any](s *Session, work func() (T, error), done func(T, error)) {
	go func() {
		result, err := work()
		s.onUI(func() { done(result, err) })
	}()
}
