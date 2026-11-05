//go:build rig

package rigtest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJourneyClipboardShortcutNeverPaintsTheRoot(t *testing.T) {
	app := Launch(t, Options{})
	clipboardText := uniqueClipboardText("shortcut destination")
	must(t, app.ClipboardSet(clipboardText))
	must(t, app.SummonPalette())
	must(t, app.Type("Clipboard History"))
	waitForRow(t, app, "Clipboard History", elementWait)
	must(t, app.Key(actionMenuKey))
	_, err := app.WaitFor("Set keyboard shortcut", elementWait)
	must(t, err)
	must(t, app.Type("Set keyboard shortcut"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Press a keyboard shortcut", elementWait)
	must(t, err)
	const shortcut = "Control+Alt+Shift+H"
	must(t, app.PressHotkey(shortcut))
	_, err = app.WaitFor("Save shortcut for “Clipboard History”", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	hidePalette(t, app)
	for _, state := range []string{"cold", "warm"} {
		t.Run(state, func(t *testing.T) {
			baseline := palettePresentations(t, app)
			window, err := app.PaletteWindow()
			must(t, err)
			pressed := make(chan error, 1)
			go func() { pressed <- app.PressHotkey(shortcut) }()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			deadline := time.After(elementWait)
			var frames []Tree
			settled := false
			for !settled {
				select {
				case err := <-pressed:
					must(t, err)
					pressed = nil
				case <-deadline:
					t.Fatalf("shortcut did not reach Clipboard History; presentations=%v; log=%s", palettePresentations(t, app), tail(app.Log(), 8))
				case <-ticker.C:
					current, err := app.PaletteWindow()
					must(t, err)
					if !current.Visible || len(palettePresentations(t, app)) <= len(baseline) {
						continue
					}
					raw, err := app.client.call("tree", map[string]any{"window": window.ID})
					must(t, err)
					var tree Tree
					must(t, json.Unmarshal(raw, &tree))
					frames = append(frames, tree)
					for _, node := range tree.Nodes {
						if node.Placeholder != "" && !strings.Contains(node.Placeholder, "Clipboard History") {
							t.Fatalf("shortcut painted an unrelated input before the target: %q", node.Placeholder)
						}
						if node.Role == "listItem" && node.Label == clipboardText {
							settled = true
							events := palettePresentations(t, app)
							var presented struct {
								Title   string
								Opacity float64
							}
							must(t, json.Unmarshal([]byte(events[len(events)-1]), &presented))
							if presented.Title != "Clipboard History" || presented.Opacity != 1 {
								t.Fatalf("revealed the wrong native frame: %+v", presented)
							}
						}
					}
				}
			}
			data, err := json.MarshalIndent(frames, "", "  ")
			must(t, err)
			must(t, os.WriteFile(app.artifactPath("shortcut-"+state+"-frames.json"), data, 0o644))
			t.Logf("sampled %d visible target-only frames", len(frames))
			shoot(t, app, "shortcut-"+state+"-target")
			must(t, app.Type("destination"))
			waitForRow(t, app, clipboardText, elementWait)
			must(t, app.Key(actionMenuKey))
			_, err = app.WaitFor("Copy", elementWait)
			must(t, err)
			must(t, app.Key("Escape"))
			hidePalette(t, app)
		})
	}
}

func palettePresentations(t *testing.T, app *App) []string {
	t.Helper()
	data, err := os.ReadFile(app.systemLog() + ".presentations")
	if os.IsNotExist(err) {
		return nil
	}
	must(t, err)
	return strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })
}
