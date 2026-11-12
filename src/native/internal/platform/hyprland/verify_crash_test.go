package hyprland

import (
	"context"
	"strings"
	"testing"
)

// Some Hyprland versions crash in --verify-config even for a valid config. A crash is no verdict: the change goes
// ahead under the reload check, and the crashing check is not run again in this process.
func TestACrashingConfigCheckDoesNotBlockApplyAndIsNotRepeated(t *testing.T) {
	verifyCrashed.Store(false)
	t.Cleanup(func() { verifyCrashed.Store(false) })
	f := newFixture(t, "0.57.0", map[string]string{"hyprland.lua": "-- mine\n"})
	f.fake.verifyExit = 139
	plan, _ := Prepare(Detect(context.Background(), f.env), desired)
	if _, err := Apply(context.Background(), f.env, plan, true); err != nil {
		t.Fatalf("a crashing check blocked the change: %v", err)
	}
	if f.fake.reloads != 1 {
		t.Fatalf("reloads = %d, want the reload check to run", f.fake.reloads)
	}
	checks := 0
	for _, call := range f.fake.calls {
		if strings.HasPrefix(call, "Hyprland --verify-config") {
			checks++
		}
	}
	again, _ := Prepare(Detect(context.Background(), f.env), Desired{Binds: []Bind{{Accelerator: "Control+Alt+J", Command: exe + " run palette-toggle"}}})
	if _, err := Apply(context.Background(), f.env, again, true); err != nil {
		t.Fatal(err)
	}
	for _, call := range f.fake.calls {
		if strings.HasPrefix(call, "Hyprland --verify-config") {
			checks--
		}
	}
	if checks != 0 {
		t.Fatal("the crashing check ran again")
	}
}
