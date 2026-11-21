//go:build rig

package rigtest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJourneyTypingKeepsTheResultSurfaceMounted(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			app := Launch(t, Options{Theme: theme})
			must(t, app.SummonPalette())
			must(t, app.Type("hello there"))
			_, err := app.WaitFor(`Automate "hello there"`, elementWait)
			must(t, err)
			window, err := app.PaletteWindow()
			must(t, err)
			shoot(t, app, "typing-before")
			typed := make(chan error, 1)
			go func() { typed <- app.Type(" the angel from") }()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			deadline := time.After(commandTimeout)
			var frames []Tree
			var typeErr error
		collect:
			for {
				select {
				case typeErr = <-typed:
					break collect
				case <-deadline:
					t.Fatal("typing did not finish")
				case <-ticker.C:
					raw, err := app.client.call("tree", map[string]any{"window": window.ID})
					must(t, err)
					var tree Tree
					must(t, json.Unmarshal(raw, &tree))
					frames = append(frames, tree)
				}
			}
			must(t, typeErr)
			final, err := app.Tree()
			must(t, err)
			frames = append(frames, final)
			data, err := json.MarshalIndent(frames, "", "  ")
			must(t, err)
			must(t, os.WriteFile(app.artifactPath("typing-frames.json"), data, 0o644))
			ids := map[string]bool{}
			empty := 0
			for _, frame := range frames {
				rows := 0
				for _, node := range frame.Nodes {
					if node.Role != "listItem" {
						continue
					}
					rows++
					if strings.HasPrefix(node.Label, `Automate "hello there`) {
						ids[node.ID] = true
					}
				}
				if rows == 0 {
					empty++
				}
			}
			shoot(t, app, "typing-after")
			t.Logf("sampled %d frames: %d blank result surfaces, %d Automate control identities", len(frames), empty, len(ids))
			if empty != 0 || len(ids) != 1 {
				t.Fatalf("typing remounted or blanked the result surface: blanks=%d control identities=%d", empty, len(ids))
			}
			waitSearchValue(t, app, "hello there the angel from")
		})
	}
}
