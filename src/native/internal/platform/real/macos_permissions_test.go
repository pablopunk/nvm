package real

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const (
	helperOK     = `{"ok":true,"code":"ok"}`
	helperDenied = `{"ok":false,"code":"denied","message":"Nevermind does not have Accessibility access"}`
)

// newScriptedMacHelper returns a helper bridge over a fake helper binary, so the window helper's answers are scripted.
func newScriptedMacHelper(scripts ...fake.HelperScript) (macHelper, *fake.Helper) {
	helper := fake.NewHelper()
	helper.Script("/unused/macos-selected-text", scripts...)
	return macHelper{helper: helper, binary: "/unused/macos-selected-text"}, helper
}

func TestAccessibilityStateReadsTheHelperWithoutPrompting(t *testing.T) {
	tests := map[string]struct {
		script fake.HelperScript
		want   platform.WindowAccess
		err    bool
	}{
		"granted":        {fake.HelperScript{Stdout: []byte(helperOK)}, platform.WindowAccessAllowed, false},
		"refused":        {fake.HelperScript{Stdout: []byte(helperDenied), ExitCode: 2}, platform.WindowAccessDenied, false},
		"unreadable":     {fake.HelperScript{Stdout: []byte("crash")}, platform.WindowAccessUnknown, true},
		"helper missing": {fake.HelperScript{Err: errors.New("no such file")}, platform.WindowAccessUnknown, true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bridge, helper := newScriptedMacHelper(test.script)
			got, err := bridge.accessState(context.Background())
			if got != test.want || (err != nil) != test.err {
				t.Fatalf("accessState() = %v, %v; want %v, error %v", got, err, test.want, test.err)
			}
			calls := helper.Calls()
			if len(calls) != 1 || len(calls[0].Args) != 2 || calls[0].Args[1] != "permissions" {
				t.Fatalf("calls = %+v, want one `window permissions`", calls)
			}
		})
	}
}

func TestRequestAccessShowsThePromptThroughTheHelper(t *testing.T) {
	for name, stdout := range map[string]string{"granted": helperOK, "refused": helperDenied} {
		t.Run(name, func(t *testing.T) {
			bridge, helper := newScriptedMacHelper(fake.HelperScript{Stdout: []byte(stdout)})
			if err := bridge.requestAccess(context.Background()); err != nil {
				t.Fatalf("requestAccess() = %v; a granted or refused prompt is not an error", err)
			}
			calls := helper.Calls()
			if len(calls) != 1 || len(calls[0].Args) != 2 || calls[0].Args[1] != accessRequestSubcommand {
				t.Fatalf("calls = %+v, want one `window request-access`", calls)
			}
		})
	}
}

func TestRequestAccessIsAnErrorWhenTheHelperCannotAnswer(t *testing.T) {
	bridge, _ := newScriptedMacHelper(fake.HelperScript{Stdout: []byte("not json")})
	if err := bridge.requestAccess(context.Background()); err == nil {
		t.Fatal("requestAccess() = nil for an unreadable answer")
	}
}

func TestMacPermissionsWiresAccessibilityAndMicrophoneOnly(t *testing.T) {
	granted := helperOK
	bridge, helper := newScriptedMacHelper(fake.HelperScript{Stdout: []byte(granted)}, fake.HelperScript{Stdout: []byte(helperOK)})
	permissions := macPermissions{access: bridge}

	state, err := permissions.State(platform.PermissionAccessibility)
	if err != nil || state != platform.PermissionAllowed {
		t.Fatalf("State(accessibility) = %v, %v; want allowed", state, err)
	}
	if state, err := permissions.State(platform.PermissionScreenRecording); state != platform.PermissionUnknown || !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("State(camera) = %v, %v; want unknown and unsupported", state, err)
	}
	if err := permissions.Request(platform.PermissionScreenRecording); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Request(screen recording) = %v; want unsupported", err)
	}
	if err := permissions.Request(platform.PermissionAccessibility); err != nil {
		t.Fatalf("Request(accessibility) = %v", err)
	}
	if calls := helper.Calls(); len(calls) != 2 {
		t.Fatalf("helper calls = %d; unsupported kinds must not reach the helper", len(calls))
	}
}

func TestPermissionStateFollowsTheWindowAccessAnswer(t *testing.T) {
	tests := map[platform.WindowAccess]platform.PermissionState{
		platform.WindowAccessAllowed: platform.PermissionAllowed,
		platform.WindowAccessDenied:  platform.PermissionDenied,
		platform.WindowAccessUnknown: platform.PermissionUnknown,
	}
	for access, want := range tests {
		if got := permissionStateOf(access); got != want {
			t.Errorf("permissionStateOf(%v) = %v, want %v", access, got, want)
		}
	}
}

func TestChooseMacosHelperPathPrefersTheBundledHelper(t *testing.T) {
	tests := []struct {
		name, bundled, executableDir, want string
	}{
		{"bundled", "/App.app/Contents/Resources/macos-selected-text", "/App.app/Contents/MacOS", "/App.app/Contents/Resources/macos-selected-text"},
		{"beside the executable", "", "/work/bin", "/work/bin/macos-selected-text"},
		{"bare name", "", "", "macos-selected-text"},
	}
	for _, test := range tests {
		if got := chooseMacosHelperPath(test.bundled, test.executableDir); got != test.want {
			t.Errorf("%s: chooseMacosHelperPath = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestWindowAccessReportsAHelperThatCannotAnswer(t *testing.T) {
	bridge, _ := newScriptedMacHelper(fake.HelperScript{Stdout: []byte("not json")})
	access, err := (macWindows{macHelper: bridge}).Access(context.Background())
	if access != platform.WindowAccessUnknown || err == nil {
		t.Fatalf("Access() = %v, %v; want unknown with the helper's failure", access, err)
	}
}
