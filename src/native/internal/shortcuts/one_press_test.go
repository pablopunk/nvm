package shortcuts

import (
	"reflect"
	"testing"
)

// TestOnePressRunsExactlyOneAction: Fix Selected Text on Command+Control+Alt+R and a window action on a neighbouring
// key never run together, and two actions asking for one key leave one claim, so a press cannot dispatch twice.
func TestOnePressRunsExactlyOneAction(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	bindings, ran := bindingsWith(hotkeys)
	bindings.Apply(PlanBindings([]Binding{
		{ActionID: "fix", Accelerator: "Command+Control+Alt+R", Title: "Fix"},
		{ActionID: "center", Accelerator: "Command+Control+Alt+C", Title: "Center"},
		{ActionID: "almost-maximize", Accelerator: "Command+Control+Alt+R", Title: "Almost Maximize"},
	}, func(string) bool { return false }))

	hotkeys.press("Command+Control+Alt+R")

	if !reflect.DeepEqual(*ran, []string{"almost-maximize"}) && !reflect.DeepEqual(*ran, []string{"fix"}) {
		t.Fatalf("a press ran %v, want exactly one action", *ran)
	}
	if len(hotkeys.registered) != 2 {
		t.Fatalf("registered = %v, want one registration per accelerator", hotkeys.registered)
	}
}
