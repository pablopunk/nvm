package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fixtureText is what the probes type and paste, so a test can read it back from the fake paster.
const fixtureText = "nvm paste fixture"

// The two probe rows. The typing row reports what the keyboard service answered; the paste row returns a paste action
// the host performs when the user runs it.
const (
	typeQuery  = "type"
	pasteQuery = "paste"
)

// SearchItems answers the typing probe through the keyboard service, and offers the paste action as a row's primary.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	switch query {
	case typeQuery:
		return []api.Item{{ID: typeQuery, Title: typeCheck(ctx)}}, nil
	case pasteQuery:
		action := api.Paste("Paste fixture", api.ClipboardContent{Type: "text", Text: fixtureText}, api.PasteOptions{})
		return []api.Item{{ID: pasteQuery, Title: "paste fixture", Primary: &action}}, nil
	}
	return nil, nil
}

// typeCheck types the fixture text and reports whether the host let it through.
func typeCheck(ctx api.Context) string {
	err := ctx.Desktop().Keyboard().TypeText(fixtureText, api.TypeOptions{})
	return fmt.Sprintf("type sent=%t", err == nil)
}

// Commands exercises a background command returning a declarative paste, as dictation does after transcription.
func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "paste-background", Title: "Background paste fixture", Mode: api.ModeBackground,
		GlobalShortcut: "Control+Alt+Shift+P", DismissAfterRun: true}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "paste-background" {
		return api.Result{}, api.ErrInvalidInput
	}
	ctx.Logs().Log(api.LogInfo, "background paste fixture invoked", nil)
	paste := api.PasteText("Paste background result", fixtureText)
	paste.Paste = &api.PasteOptions{Concealed: true, RestoreClipboard: true}
	return api.Result{Next: &paste}, nil
}
