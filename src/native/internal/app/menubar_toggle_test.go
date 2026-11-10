package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

func TestMenuBarTogglePreviewRowRunsTheHostToggle(t *testing.T) {
	item := menuBarTogglePreviewItem(manager.MenuBarExtension{ID: "user.issues", Title: "GitHub Issues"})
	if item.Title != "Toggle GitHub Issues in Menu Bar" || item.Primary == nil || item.Primary.Op != api.HostToggleMenuBar {
		t.Fatalf("preview row = %+v", item)
	}
	if err := item.Primary.Validate(); err != nil {
		t.Fatalf("the host refuses the toggle action: %v", err)
	}
}

func TestMenuBarToggleRowSaysWhatItWillDo(t *testing.T) {
	p := &App{}
	extension := manager.MenuBarExtension{ID: "user.issues", Title: "GitHub Issues"}

	row := p.menuBarToggleCommand(extension)
	if row.Title != "Toggle GitHub Issues in Menu Bar" || row.ID != "extension-menubar:user.issues" {
		t.Fatalf("shown row = %q (%q)", row.Title, row.ID)
	}

	if len(row.extraActions) != 1 || row.extraActions[0].Title != "Tweak with AI" {
		t.Fatalf("the toggle row's menu = %+v, want Tweak with AI", row.extraActions)
	}

	p.visibility.setShown("user.issues", false)
	row = p.menuBarToggleCommand(extension)
	if row.Title != "Toggle GitHub Issues in Menu Bar" || row.Subtitle != "Hidden from the menu bar" {
		t.Fatalf("hidden row = %q, %q", row.Title, row.Subtitle)
	}
}
