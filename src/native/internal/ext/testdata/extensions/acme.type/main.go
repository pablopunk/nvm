package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fixtureText is what the probe types, so a test can read it back from the fake paster.
const fixtureText = "nvm paste fixture"

// typeQuery is the probe row. Typing text is macOS only (api.PlatformOperations), so this fixture is too; the paste
// fixture (acme.paste) stays on every OS.
const typeQuery = "type"

// SearchItems answers the typing probe through the keyboard service.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query != typeQuery {
		return nil, nil
	}
	return []api.Item{{ID: typeQuery, Title: typeCheck(ctx)}}, nil
}

// typeCheck types the fixture text and reports whether the host let it through.
func typeCheck(ctx api.Context) string {
	err := ctx.Desktop().Keyboard().TypeText(fixtureText, api.TypeOptions{})
	return fmt.Sprintf("type sent=%t", err == nil)
}
