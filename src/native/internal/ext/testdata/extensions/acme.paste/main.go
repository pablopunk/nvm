package main

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fixtureText is what the probes paste, so a test can read it back from the fake paster.
const fixtureText = "nvm paste fixture"

// pasteQuery is the probe row: it returns a paste action the host performs when the user runs it. The fixture calls no
// macOS-only operation, so it runs on every OS; the typing probe is acme.type.
const pasteQuery = "paste"

// SearchItems offers the paste action as a row's primary.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query != pasteQuery {
		return nil, nil
	}
	action := api.Paste("Paste fixture", api.ClipboardContent{Type: "text", Text: fixtureText}, api.PasteOptions{})
	return []api.Item{{ID: pasteQuery, Title: "paste fixture", Primary: &action}}, nil
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
