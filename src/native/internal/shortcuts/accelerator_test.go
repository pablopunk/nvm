package shortcuts

import (
	"reflect"
	"testing"
)

// Ported from shortcut-utils.test.ts: "shortcut normalization remains deterministic across platform wording".
func TestNormalizeResolvesEveryWordingToOneCanonicalForm(t *testing.T) {
	for in, want := range map[string]string{
		" ctrl + option + k ": "Control+Alt+K",
		"cmd+option+k":        "Command+Alt+K",
		"CmdOrCtrl+enter":     "CommandOrControl+Enter",
		"Command+Alt+K":       "Command+Alt+K",
		"  ":                  "",
		"Shift+Return":        "Shift+Enter",
		"opt+esc":             "Alt+Escape",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPartsSplitsTheCanonicalForm(t *testing.T) {
	if got, want := Parts("cmd+alt+k"), []string{"Command", "Alt", "K"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Parts = %v, want %v", got, want)
	}
	if Parts("") != nil {
		t.Fatal("Parts of a blank accelerator should be nil")
	}
}
