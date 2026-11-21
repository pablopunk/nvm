package api

import "testing"

func TestCommandsAreCustomizableUnlessTheyOptOut(t *testing.T) {
	off := false
	if !(Command{ID: "a", Title: "A"}).TakesCustomization() {
		t.Fatal("a command that says nothing must take shortcut and alias actions")
	}
	if (Command{ID: "a", Title: "A", Customizable: &off}).TakesCustomization() {
		t.Fatal("NotCustomizable must opt a command out")
	}
}
