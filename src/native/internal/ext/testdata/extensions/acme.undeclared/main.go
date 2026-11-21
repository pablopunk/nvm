package main

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems runs every desktop probe without declaring any capability. The row counts how many the host refused
// for permission, so the fixture shows the refusals instead of a tally of successes.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query != "undeclared" {
		return nil, nil
	}
	refused := 0
	probes := undeclaredProbes(ctx)
	for _, probe := range probes {
		if errors.Is(probe(), api.ErrPermission) {
			refused++
		}
	}
	return []api.Item{{ID: query, Title: fmt.Sprintf("undeclared refused=%d of %d", refused, len(probes))}}, nil
}

// undeclaredProbes lists one call per desktop service that reports errors, each returned as a closure over ctx.
func undeclaredProbes(ctx api.Context) []func() error {
	return []func() error{
		func() error { _, err := ctx.Desktop().Clipboard().ReadText(); return err },
		func() error { _, err := ctx.Desktop().Apps().List(); return err },
		func() error { _, err := ctx.Desktop().WindowManagement().Windows(api.WindowQuery{}); return err },
		func() error { _, err := ctx.Clipboard().List(api.ClipboardQuery{}); return err },
		func() error {
			_, err := ctx.OCR().Image(api.OCRInput{Source: "/tmp/x.png"}, api.OCROptions{})
			return err
		},
		func() error { return ctx.Desktop().Files().Open("/tmp/x") },
		func() error { return ctx.Desktop().Shell().OpenExternal("https://example.com") },
	}
}
