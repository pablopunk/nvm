package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestFormFooterNamesTheSubmitKeyOfEachPlatform(t *testing.T) {
	cases := []struct {
		goos string
		mod  ui.Modifiers
		hint string
	}{
		{goos: "darwin", mod: ui.Cmd, hint: "⌘↵ Save   esc Cancel"},
		{goos: "linux", mod: ui.Ctrl, hint: "⌃↵ Save   esc Cancel"},
		{goos: "windows", mod: ui.Ctrl, hint: "⌃↵ Save   esc Cancel"},
	}
	for _, c := range cases {
		if got := formFooterHintOn(c.goos); got != c.hint {
			t.Errorf("%s form footer = %q, want %q", c.goos, got, c.hint)
		}
		if got := palette.ModOn(c.goos); got != c.mod {
			t.Errorf("%s submit modifier = %v, want %v", c.goos, got, c.mod)
		}
	}
}
