package shortcuts

import (
	"reflect"
	"testing"
)

var noneReserved = func(string) bool { return false }

// Ported from shortcut-binding-plan.test.ts: "duplicate user accelerators bind the first action and mark the second
// inactive".
func TestDuplicateUserAcceleratorsBindTheFirstActionByID(t *testing.T) {
	plan := PlanBindings([]Binding{
		{ActionID: "extension-action:a:two", Accelerator: "Command+Shift+X", Title: "Two", Source: SourceUser},
		{ActionID: "extension-action:a:one", Accelerator: "Command+Shift+X", Title: "One", Source: SourceUser},
	}, noneReserved)
	if want := []Claim{{"extension-action:a:one", "Command+Shift+X"}}; !reflect.DeepEqual(plan.Claims, want) {
		t.Fatalf("claims = %v, want %v", plan.Claims, want)
	}
	if len(plan.Inactive) != 1 || plan.Inactive[0].ActionID != "extension-action:a:two" {
		t.Fatalf("inactive = %+v", plan.Inactive)
	}
	if got := plan.Inactive[0].Message; got != "Command+Shift+X is already used by One" {
		t.Fatalf("message = %q", got)
	}
}

// Ported from shortcut-binding-plan.test.ts: "user shortcuts win over declared ones and reserved accelerators stay
// inactive".
func TestUserShortcutsWinOverDeclaredAndReservedStaysInactive(t *testing.T) {
	plan := PlanBindings([]Binding{
		{ActionID: "a", Accelerator: "Command+K", Title: "Declared", Source: SourceDeclared},
		{ActionID: "b", Accelerator: "Command+K", Title: "User", Source: SourceUser},
		{ActionID: "c", Accelerator: "Command+Alt+K", Title: "Reserved", Source: SourceUser},
	}, func(accelerator string) bool { return accelerator == "Command+Alt+K" })
	if want := []Claim{{"b", "Command+K"}}; !reflect.DeepEqual(plan.Claims, want) {
		t.Fatalf("claims = %v, want %v", plan.Claims, want)
	}
	messages := map[string]string{}
	for _, item := range plan.Inactive {
		messages[item.ActionID] = item.Message
	}
	if messages["a"] != "Command+K is already used by User" {
		t.Errorf("declared message = %q", messages["a"])
	}
	if messages["c"] != "Command+Alt+K is reserved for opening Nevermind" {
		t.Errorf("reserved message = %q", messages["c"])
	}
}

// Ported from shortcut-binding-plan.test.ts: "declared shortcuts are hidden by a user override or a disabled marker, and
// dormant IDs are skipped".
func TestDeclaredShortcutsHiddenByChoiceOrRemovalAndDormantIDsSkipped(t *testing.T) {
	wanted := Wanted(
		map[string]string{"extension-action:a:overridden": "Command+Shift+O", "extension-action:gone:missing": "Command+Shift+M"},
		map[string]bool{"extension-action:a:disabled": true},
		map[string]string{"extension-action:a:overridden": "Overridden", "extension-action:a:disabled": "Disabled", "extension-action:a:declared": "Declared"},
		[]Declared{
			{ActionID: "extension-action:a:overridden", Accelerator: "Command+Shift+Z", Title: "Overridden"},
			{ActionID: "extension-action:a:disabled", Accelerator: "Command+Shift+D", Title: "Disabled"},
			{ActionID: "extension-action:a:declared", Accelerator: "Command+Shift+Q", Title: "Declared"},
		},
	)
	var got []string
	for _, b := range wanted {
		got = append(got, sourceName(b.Source)+":"+b.ActionID)
	}
	want := []string{"user:extension-action:a:overridden", "declared:extension-action:a:declared"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wanted = %v, want %v", got, want)
	}
}

func TestClaimOrderIsIndependentOfInputOrder(t *testing.T) {
	forward := []Binding{
		{ActionID: "z", Accelerator: "Command+Shift+1", Title: "Z", Source: SourceUser},
		{ActionID: "a", Accelerator: "Command+Shift+1", Title: "A", Source: SourceUser},
	}
	reversed := []Binding{forward[1], forward[0]}
	if !reflect.DeepEqual(PlanBindings(forward, noneReserved), PlanBindings(reversed, noneReserved)) {
		t.Fatal("the plan changes with the order of the wanted bindings")
	}
}

func sourceName(source Source) string {
	if source == SourceUser {
		return "user"
	}
	return "declared"
}
