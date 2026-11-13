package aichats

import (
	"fmt"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Refs of the 1.x converter. The path of the file to convert travels under argPath.
const (
	commandConvert = "convert-1x-extension"
	opConvert      = "convert"
	argPath        = "path"
)

// legacyExtensionsRoot is where 1.x extensions live; 2.x folders are found there too, but as folders.
const legacyExtensionsRoot = "~/.config/nevermind/extensions"

// convertView lists the 1.x extension files that are still in the extensions folder.
func convertView(ctx api.Context) (*api.View, error) {
	found, err := ctx.Desktop().Files().Find([]string{legacyExtensionsRoot}, api.FindOptions{Depth: 1, Extensions: []string{"ts", "js", "mjs", "cjs"}})
	if err != nil {
		return nil, err
	}
	items := make([]api.Item, 0, len(found))
	for _, file := range found {
		primary := api.Run(opConvert, "Convert", map[string]string{argPath: file.Path})
		items = append(items, api.Item{ID: file.Path, Title: filepath.Base(file.Path), Subtitle: file.DisplayPath, Icon: "file-text", Primary: &primary})
	}
	view := api.ListView("Convert 1.x Extension", items...)
	view.EmptyView = &api.EmptyState{Title: "No 1.x extensions", Subtitle: "There are no older extension files in the extensions folder."}
	return &view, nil
}

// startConversion opens a builder chat that rewrites the file in Go.
func startConversion(ctx api.Context, path string) (api.Result, error) {
	source, err := ctx.Desktop().Files().ReadText(path)
	if err != nil {
		return api.Result{}, err
	}
	name := filepath.Base(path)
	return next(ctx.AIBuilder().StartChat(conversionPrompt(name, source), "Convert "+name)), nil
}

func conversionPrompt(name, source string) string {
	return fmt.Sprintf("Convert this 1.x Nevermind extension to Go. Call read_1x_mapping for what each old name became, keep "+
		"its behavior, and set convertedFrom to %q in the manifest.\n\nFile %s:\n\n~~~\n%s\n~~~", name, name, source)
}
