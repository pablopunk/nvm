package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems answers one query per desktop service. Each row's title says what the host answered, so a rig journey
// can read the outcome from the palette.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query == clearQuery {
		return []api.Item{{ID: clearActionID, Title: "Fixture clear history", Primary: &api.Action{Kind: api.ActionRun, Title: "Fixture clear history", Ref: &api.ActionRef{ID: clearActionID}}}}, nil
	}
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

var checks = map[string]func(api.Context) string{
	"clipboard": clipboardCheck,
	"windows":   windowsCheck,
	"apps":      appsCheck,
	"history":   historyCheck,
	"shortcuts": shortcutsCheck,
	"system":    systemCheck,
	"ocr":       ocrCheck,
	"files":     filesCheck,
	"links":     linksCheck,
}

const fixtureClipboardText = "nvm desktop fixture"

// clearQuery and clearActionID name the row that clears clipboard history. The fixture does not declare a
// confirmation: the host must ask the user anyway, which is what the rig journey checks.
const (
	clearQuery    = "fixture clear"
	clearActionID = "clear-history"
)

// RunAction clears clipboard history when the user runs the clear row. The host asks before the call runs.
func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != clearActionID {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	_, err := ctx.Clipboard().Clear(nil, 0)
	return api.Result{}, err
}

func clipboardCheck(ctx api.Context) string {
	if err := ctx.Desktop().Clipboard().WriteText(fixtureClipboardText, false); err != nil {
		return "clipboard write failed: " + err.Error()
	}
	text, err := ctx.Desktop().Clipboard().ReadText()
	if err != nil {
		return "clipboard read failed: " + err.Error()
	}
	return "clipboard read=" + text
}

func windowsCheck(ctx api.Context) string {
	windows, err := ctx.Desktop().WindowManagement().Windows(api.WindowQuery{})
	if err != nil {
		return "windows failed: " + err.Error()
	}
	return fmt.Sprintf("windows listed=%d", len(windows))
}

func appsCheck(ctx api.Context) string {
	apps, err := ctx.Desktop().Apps().List()
	if err != nil {
		return "apps failed: " + err.Error()
	}
	return fmt.Sprintf("apps listed=%t", len(apps) > 0)
}

func historyCheck(ctx api.Context) string {
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Limit: 10})
	if err != nil {
		return "history failed: " + err.Error()
	}
	return fmt.Sprintf("history entries=%d", len(entries))
}

func shortcutsCheck(ctx api.Context) string {
	return "shortcuts palette=" + ctx.Shortcuts().Palette().Accelerator
}

func systemCheck(ctx api.Context) string {
	return fmt.Sprintf("system os=%s ocr=%t", ctx.System().OS(), ctx.System().HasCapability("ocr"))
}

// ocrCheck reads a file that is not an image, which the host refuses before any engine runs.
func ocrCheck(ctx api.Context) string {
	_, err := ctx.OCR().Image(api.OCRInput{Source: "/etc/hostname"}, api.OCROptions{})
	return fmt.Sprintf("ocr refused=%t", err != nil)
}

// filesCheck asks to open a relative path, which the host refuses.
func filesCheck(ctx api.Context) string {
	err := ctx.Desktop().Files().Open("notes.txt")
	return fmt.Sprintf("files refused=%t", err != nil)
}

// linksCheck asks to open a file URL as a link, which the host refuses before any browser starts.
func linksCheck(ctx api.Context) string {
	err := ctx.Desktop().Shell().OpenExternal("file:///etc/hostname")
	return fmt.Sprintf("links refused=%t", err != nil)
}
