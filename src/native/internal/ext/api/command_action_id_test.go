package api

import "testing"

func TestCommandActionIDMustEndWithTheCommandID(t *testing.T) {
	ok := Command{ID: "layout.left-half", ActionID: "extension-action:window-management:layout.left-half", Title: "Left Half"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want the 1.x key accepted", err)
	}
	wrong := Command{ID: "layout.left-half", ActionID: "extension-action:window-management:left", Title: "Left Half"}
	if err := wrong.Validate(); err == nil {
		t.Fatal("Validate() accepted an actionId that does not name the command")
	}
}
