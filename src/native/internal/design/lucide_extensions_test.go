package design

import "testing"

// TestIconsTheConvertedExtensionsNameAreKnown checks every icon a converted 1.x extension asks for has a drawing, so
// its rows do not all show the puzzle piece the palette draws for an unknown name.
func TestIconsTheConvertedExtensionsNameAreKnown(t *testing.T) {
	for _, name := range []string{
		"arrow-right-left", "calendar-days", "circle-check", "circle-dot", "circle-lefthalf-fill", "circle-x", "clapperboard", "coffee", "fast-forward", "file", "folder-tree", "git-pull-request", "github", "grid", "hard-drive", "laptop", "network", "panels-top-left", "power-off", "refresh-cw", "rotate-cw", "scan-search", "shield-off", "shrink", "skull", "text", "text-cursor-input", "undo-2", "volume-2", "wifi", "wifi-off",
	} {
		if _, ok := iconSources[name]; !ok {
			t.Errorf("icon %q has no drawing", name)
		}
	}
}
