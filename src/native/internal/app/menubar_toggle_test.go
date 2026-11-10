package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

func TestMenuBarToggleRowSaysWhatItWillDo(t *testing.T) {
	p := &App{}
	extension := manager.MenuBarExtension{ID: "user.issues", Title: "GitHub Issues"}

	row := p.menuBarToggleCommand(extension)
	if row.Title != "Hide GitHub Issues from Menu Bar" || row.ID != "extension-menubar:user.issues" {
		t.Fatalf("shown row = %q (%q)", row.Title, row.ID)
	}

	if len(row.extraActions) != 1 || row.extraActions[0].Title != "Tweak with AI" {
		t.Fatalf("the toggle row's menu = %+v, want Tweak with AI", row.extraActions)
	}

	p.visibility.setShown("user.issues", false)
	row = p.menuBarToggleCommand(extension)
	if row.Title != "Show GitHub Issues in Menu Bar" || row.Subtitle != "Hidden from the menu bar" {
		t.Fatalf("hidden row = %q, %q", row.Title, row.Subtitle)
	}
}
