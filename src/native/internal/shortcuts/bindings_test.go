package shortcuts

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// recordingHotkeys records what is bound and released. refused accelerators fail as the OS refuses a key another
// program holds.
type recordingHotkeys struct {
	registered   []string
	unregistered []string
	refused      map[string]bool
	bound        map[string]func()
}

func newRecordingHotkeys(refused ...string) *recordingHotkeys {
	h := &recordingHotkeys{refused: map[string]bool{}, bound: map[string]func(){}}
	for _, accelerator := range refused {
		h.refused[accelerator] = true
	}
	return h
}

func (h *recordingHotkeys) Register(accelerator string, fn func()) error {
	if h.refused[accelerator] {
		return platform.ErrConflict
	}
	if _, taken := h.bound[accelerator]; taken {
		return platform.ErrConflict
	}
	h.registered = append(h.registered, accelerator)
	h.bound[accelerator] = fn
	return nil
}

func (h *recordingHotkeys) Unregister(accelerator string) {
	h.unregistered = append(h.unregistered, accelerator)
	delete(h.bound, accelerator)
}

func (h *recordingHotkeys) press(accelerator string) {
	h.bound[accelerator]()
}

func bindingsWith(hotkeys platform.Hotkeys) (*Bindings, *[]string) {
	var ran []string
	bindings := NewBindings(hotkeys, func(actionID string) { ran = append(ran, actionID) })
	return bindings, &ran
}

// Ported from shortcut-bindings: "releasing one action never unregisters another action's binding".
func TestReleasingOneActionNeverUnregistersAnother(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	bindings, _ := bindingsWith(hotkeys)
	bindings.Apply(PlanBindings([]Binding{
		{ActionID: "a", Accelerator: "Command+Shift+1", Title: "A"},
		{ActionID: "b", Accelerator: "Command+Shift+2", Title: "B"},
	}, noneReserved))
	bindings.Apply(PlanBindings([]Binding{{ActionID: "b", Accelerator: "Command+Shift+2", Title: "B"}}, noneReserved))
	if want := []string{"Command+Shift+1"}; !reflect.DeepEqual(hotkeys.unregistered, want) {
		t.Fatalf("unregistered = %v, want %v", hotkeys.unregistered, want)
	}
	if want := []string{"Command+Shift+1", "Command+Shift+2"}; !reflect.DeepEqual(hotkeys.registered, want) {
		t.Fatalf("registered = %v, want %v", hotkeys.registered, want)
	}
}

// Ported from shortcut-bindings: "a binding the OS refuses is reported as a failure and not tracked as active".
func TestAnOSRefusalIsReportedAndNotTracked(t *testing.T) {
	bindings, _ := bindingsWith(newRecordingHotkeys("Command+Shift+9"))
	failures := bindings.Apply(PlanBindings([]Binding{{ActionID: "a", Accelerator: "Command+Shift+9", Title: "A"}}, noneReserved))
	if got := failures["a"]; got != "Command+Shift+9 is in use by another app" {
		t.Fatalf("failure = %q", got)
	}
	if _, active := bindings.Owned()["Command+Shift+9"]; active {
		t.Fatal("a refused binding is tracked as active")
	}
}

// Ported from shortcut-bindings: "a duplicate accelerator never reaches the OS registration as a second binding".
func TestADuplicateAcceleratorNeverReachesTheOSTwice(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	bindings, _ := bindingsWith(hotkeys)
	failures := bindings.Apply(PlanBindings([]Binding{
		{ActionID: "a", Accelerator: "Command+Shift+5", Title: "A"},
		{ActionID: "b", Accelerator: "Command+Shift+5", Title: "B"},
	}, noneReserved))
	if want := []string{"Command+Shift+5"}; !reflect.DeepEqual(hotkeys.registered, want) {
		t.Fatalf("registered = %v, want %v", hotkeys.registered, want)
	}
	if got := failures["b"]; got != "Command+Shift+5 is already used by A" {
		t.Fatalf("failure for b = %q", got)
	}
}

func TestAPressRunsTheActionThatOwnsTheKey(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	bindings, ran := bindingsWith(hotkeys)
	bindings.Apply(PlanBindings([]Binding{{ActionID: "clipboard", Accelerator: "Command+Alt+V", Title: "Clipboard"}}, noneReserved))
	hotkeys.press("Command+Alt+V")
	if want := []string{"clipboard"}; !reflect.DeepEqual(*ran, want) {
		t.Fatalf("ran = %v, want %v", *ran, want)
	}
}

func TestReleaseAllUnbindsEverythingItOwns(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	bindings, _ := bindingsWith(hotkeys)
	bindings.Apply(PlanBindings([]Binding{
		{ActionID: "a", Accelerator: "Command+Alt+1", Title: "A"},
		{ActionID: "b", Accelerator: "Command+Alt+2", Title: "B"},
	}, noneReserved))
	bindings.ReleaseAll()
	released := append([]string(nil), hotkeys.unregistered...)
	sort.Strings(released)
	if want := []string{"Command+Alt+1", "Command+Alt+2"}; !reflect.DeepEqual(released, want) {
		t.Fatalf("released = %v, want %v", released, want)
	}
	if len(bindings.Owned()) != 0 {
		t.Fatal("bindings remain tracked after ReleaseAll")
	}
}

func TestAnyOtherRefusalIsReportedWithItsCause(t *testing.T) {
	failed := errors.New("no portal")
	if got := refusalMessage("Command+K", failed); got != "Command+K could not be bound: no portal" {
		t.Fatalf("message = %q", got)
	}
}
