package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeSystem returns the palette's system fake, which the golden palette wires in.
func fakeSystem(t *testing.T, p *App) *fake.System {
	t.Helper()
	system, ok := p.ports.System.(*fake.System)
	if !ok {
		t.Fatalf("the palette's system port is %T, want the fake", p.ports.System)
	}
	return system
}

// systemRow finds the root row of a system command by its title, the way the palette finds it.
func systemRow(t *testing.T, p *App, title string) search.Item {
	t.Helper()
	return findRowTitled(t, collectExtensionRows(t, p, strings.ToLower(title)), title)
}

// TestRestartAsksBeforeItRestarts pushes a confirmation for a destructive command and runs nothing until the user
// confirms.
func TestRestartAsksBeforeItRestarts(t *testing.T) {
	p := goldenPalette(t)
	systemRow(t, p, "Restart").Actions[0].Run(p)
	if title := p.Top().Title; title != "Confirm" {
		t.Fatalf("top view = %q, want the confirmation", title)
	}
	if calls := fakeSystem(t, p).Calls(); len(calls) != 0 {
		t.Fatalf("Restart ran before confirmation: %+v", calls)
	}
}

// TestSystemCommandRunsOffTheUIThread performs the command through the port and reports nothing when it succeeds.
func TestSystemCommandRunsOffTheUIThread(t *testing.T) {
	p := goldenPalette(t)
	system := fakeSystem(t, p)
	p.runSystemOp("sleep", func(string) {})
	call := awaitValue(t, system.Events(), "the sleep call")
	if call.Op != fake.OpSystemSleep {
		t.Fatalf("call = %+v, want Sleep", call)
	}
}

// TestFailedSystemCommandsSayWhy explains an unsupported command plainly and keeps the OS reason otherwise.
func TestFailedSystemCommandsSayWhy(t *testing.T) {
	if got := describeSystemFailure("Lock Screen", platform.ErrUnsupported); got != "Lock Screen is not available on this system." {
		t.Fatalf("unsupported = %q", got)
	}
	if got := describeSystemFailure("Sleep", errors.New("exit status 1")); got != "Could not run sleep: exit status 1" {
		t.Fatalf("failure = %q", got)
	}
}

// TestPermissionPromptsOnlyWhereTheOSCanAsk gives Enter a prompt for a permission the OS has not asked for yet, and
// only the settings pane for one that is allowed.
func TestPermissionPromptsOnlyWhereTheOSCanAsk(t *testing.T) {
	p := goldenPalette(t)
	states := permissionFake(t, p)
	states.SetState(platform.PermissionMicrophone, platform.PermissionNotDetermined)
	states.SetState(platform.PermissionCamera, platform.PermissionAllowed)
	for _, row := range p.systemSvc.PermissionRows() {
		item := p.permissionItem(row)
		switch row.Kind {
		case platform.PermissionMicrophone:
			if len(item.Actions) != 2 || !strings.HasPrefix(item.Actions[0].Title, "Allow ") {
				t.Fatalf("microphone actions = %+v, want an Allow prompt then Open Settings", item.Actions)
			}
		case platform.PermissionCamera:
			if len(item.Actions) != 1 || !strings.HasPrefix(item.Actions[0].Title, "Open ") {
				t.Fatalf("camera actions = %+v, want only Open Settings", item.Actions)
			}
		}
	}
}

// permissionFake returns the palette's permission fake.
func permissionFake(t *testing.T, p *App) *fake.Permissions {
	t.Helper()
	states, ok := p.ports.Permissions.(*fake.Permissions)
	if !ok {
		t.Fatalf("the palette's permission port is %T, want the fake", p.ports.Permissions)
	}
	return states
}

// TestPermissionRequestWithoutAPromptSaysWhereToGo explains the settings pane, not a refusal.
func TestPermissionRequestWithoutAPromptSaysWhereToGo(t *testing.T) {
	got := permissionRequestFailure("Microphone", platform.ErrUnsupported)
	if got != "Allow microphone in System Settings. Nevermind cannot show that prompt here." {
		t.Fatalf("message = %q", got)
	}
}

// systemGoldens is the group of the OS capability goldens: the permissions, the confirmation and the guide.
var systemGoldens = golden.Options{Group: "palette", Width: width, Height: height}

func TestPaletteOSPermissions(t *testing.T) {
	p := goldenPalette(t)
	states := permissionFake(t, p)
	states.SetState(platform.PermissionMicrophone, platform.PermissionNotDetermined)
	states.SetState(platform.PermissionCamera, platform.PermissionAllowed)
	goldenOpen(p, p.permissionsView())
	golden.AssertMatrix(t, "os-permissions", p.View, systemGoldens)
}

func TestPaletteRestartConfirmation(t *testing.T) {
	p := goldenPalette(t)
	systemRow(t, p, "Restart").Actions[0].Run(p)
	golden.AssertMatrix(t, "system-confirm", p.View, systemGoldens)
}

func TestPaletteGettingStartedGuide(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.guideView())
	golden.AssertMatrix(t, "getting-started", p.View, systemGoldens)
}
