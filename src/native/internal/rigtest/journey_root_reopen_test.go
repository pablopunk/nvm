//go:build rig

package rigtest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJourneyWarmRootKeepsAppsThroughoutReopening(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	initial, err := app.Tree()
	must(t, err)
	if !treeHasApplication(initial) {
		t.Fatal("rig root has no applications to track")
	}
	for range 3 {
		hidePalette(t, app)
		window, err := app.PaletteWindow()
		must(t, err)
		pressed := make(chan error, 1)
		go func() { pressed <- app.PressHotkey(DefaultHotkey) }()
		ticker := time.NewTicker(time.Millisecond)
		deadline := time.After(350 * time.Millisecond)
		var frames []Tree
	collect:
		for {
			select {
			case err := <-pressed:
				must(t, err)
				pressed = nil
			case <-deadline:
				break collect
			case <-ticker.C:
				current, err := app.PaletteWindow()
				must(t, err)
				if !current.Visible {
					continue
				}
				raw, err := app.client.call("tree", map[string]any{"window": window.ID})
				must(t, err)
				var tree Tree
				must(t, json.Unmarshal(raw, &tree))
				frames = append(frames, tree)
				if !treeHasApplication(tree) {
					t.Fatal("warm root replaced its apps with an incomplete refresh")
				}
			}
		}
		ticker.Stop()
		if len(frames) == 0 {
			t.Fatal("no visible opening frames sampled")
		}
		data, err := json.MarshalIndent(frames, "", "  ")
		must(t, err)
		must(t, os.WriteFile(app.artifactPath("warm-root-frames.json"), data, 0o644))
		t.Logf("sampled %d app-preserving opening frames", len(frames))
	}
	shoot(t, app, "warm-root-stable")
}

func treeHasApplication(tree Tree) bool {
	for _, node := range tree.Nodes {
		if node.Role != "listItem" {
			continue
		}
		var key string
		if json.Unmarshal(node.Key, &key) == nil && strings.Contains(key, `"Applications"`) {
			return true
		}
	}
	return false
}
