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
