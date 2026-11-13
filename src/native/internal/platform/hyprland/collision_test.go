package hyprland

import (
	"context"
	"strings"
	"testing"
)

const (
	userExec = `{"modmask":12,"key":"K","dispatcher":"exec","arg":"kitty"}`
	sameExec = `{"modmask":12,"key":"K","dispatcher":"exec","arg":"` + exe + ` run palette-toggle"}`
	luaBind  = `{"modmask":12,"key":"K","dispatcher":"HL.Dispatcher(exec_cmd)","arg":"153"}`
)

func prepareAgainst(t *testing.T, f *fixture, binds string) Plan {
	t.Helper()
	f.fake.binds = "[" + binds + "]"
	plan, err := PrepareAgainstBinds(context.Background(), f.env, Detect(context.Background(), f.env), desired)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestNoCollisionWritesTheBind(t *testing.T) {
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "-- mine\n"})
	plan := prepareAgainst(t, f, `{"modmask":64,"key":"Q","dispatcher":"exec","arg":"kitty"}`)
	if len(plan.Skipped) != 0 || !strings.Contains(string(plan.ModuleAfter), "hl.bind(") {
		t.Fatalf("%+v\n%s", plan.Skipped, plan.ModuleAfter)
	}
}

func TestUserBindOnTheKeyIsSkippedAndReported(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "bind = CTRL ALT, K, exec, kitty\n"})
	plan := prepareAgainst(t, f, userExec)
	if len(plan.Skipped) != 1 || plan.Skipped[0].What != "Control+Alt+K" || !strings.Contains(plan.Skipped[0].Reason, "exec kitty") {
		t.Fatalf("%+v", plan.Skipped)
	}
	if strings.Contains(string(plan.ModuleAfter), "bind = ") {
		t.Fatalf("a colliding bind was written:\n%s", plan.ModuleAfter)
	}
	if !strings.Contains(plan.Preview(true), "Not written: Control+Alt+K") {
		t.Fatal("the preview does not name the skipped shortcut")
	}
}

func TestUserBindRunningTheSameCommandIsSilentlySatisfied(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": ""})
	plan := prepareAgainst(t, f, sameExec)
	if len(plan.Skipped) != 0 || strings.Contains(string(plan.ModuleAfter), "bind = ") {
		t.Fatalf("%+v\n%s", plan.Skipped, plan.ModuleAfter)
	}
}

func TestOwnPreviousModuleBindIsNotACollision(t *testing.T) {
	for _, tc := range []struct{ version, entry, bind string }{
		{"0.54.0", "hyprland.conf", sameExec},
		{"0.56.0", "hyprland.lua", luaBind},
	} {
		f := newFixture(t, tc.version, map[string]string{tc.entry: ""})
		applyFresh(t, f)
		plan := prepareAgainst(t, f, tc.bind)
		if len(plan.Skipped) != 0 || !plan.ModuleUnchanged() {
			t.Fatalf("%s: %+v", tc.version, plan.Skipped)
		}
	}
}

func TestOwnBindDoesNotHideAUserBindOnTheSameKey(t *testing.T) {
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": ""})
	applyFresh(t, f)
	plan := prepareAgainst(t, f, luaBind+","+luaBind)
	if len(plan.Skipped) != 1 {
		t.Fatalf("a user bind next to ours was missed: %+v", plan.Skipped)
	}
}

func TestLuaOpaqueArgumentCountsAsACollision(t *testing.T) {
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "-- bind_hyper\n"})
	plan := prepareAgainst(t, f, luaBind)
	if len(plan.Skipped) != 1 || !strings.Contains(plan.Skipped[0].Reason, "cannot read") {
		t.Fatalf("%+v", plan.Skipped)
	}
	if strings.Contains(string(plan.ModuleAfter), "hl.bind(") {
		t.Fatal("wrote a bind over a Lua bind")
	}
}
