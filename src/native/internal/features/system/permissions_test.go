package system

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// kindsOf lists the permission kinds of rows in order.
func kindsOf(rows []PermissionRow) []platform.PermissionKind {
	kinds := make([]platform.PermissionKind, 0, len(rows))
	for _, row := range rows {
		kinds = append(kinds, row.Kind)
	}
	return kinds
}

// TestPermissionRowsOnLinuxAreMicrophoneAndCamera lists the two permissions every OS has.
func TestPermissionRowsOnLinuxAreMicrophoneAndCamera(t *testing.T) {
	rows := PermissionRows("linux", fake.NewPermissions())
	want := []platform.PermissionKind{platform.PermissionMicrophone, platform.PermissionCamera}
	if got := kindsOf(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestPermissionRowsOnMacOSAddTheSystemAccesses adds accessibility, screen recording and files, all in the Privacy
// pane.
func TestPermissionRowsOnMacOSAddTheSystemAccesses(t *testing.T) {
	rows := PermissionRows("darwin", fake.NewPermissions())
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}
	for _, row := range rows {
		if row.Pane != PrivacyPaneID {
			t.Errorf("%s opens pane %q, want the Privacy pane", row.Kind, row.Pane)
		}
	}
}

// TestPermissionsThatNeedAttentionComeFirst puts denied and not-yet-asked permissions above allowed ones, keeping
// the table order inside each group.
func TestPermissionsThatNeedAttentionComeFirst(t *testing.T) {
	states := fake.NewPermissions()
	states.SetState(platform.PermissionMicrophone, platform.PermissionAllowed)
	states.SetState(platform.PermissionCamera, platform.PermissionDenied)
	got := kindsOf(PermissionRows("linux", states))
	want := []platform.PermissionKind{platform.PermissionCamera, platform.PermissionMicrophone}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestAUnreportedStateIsUnknownNeverAllowed treats a port error as unknown, even when the port says allowed.
func TestAUnreportedStateIsUnknownNeverAllowed(t *testing.T) {
	rows := PermissionRows("linux", fake.NewPermissions())
	for _, row := range rows {
		if row.State != platform.PermissionUnknown {
			t.Errorf("%s = %q, want unknown when the port cannot report it", row.Kind, row.State)
		}
	}
	allowed, known := AllowedCount(rows)
	if allowed != 0 || known != 0 {
		t.Fatalf("AllowedCount = %d of %d, want 0 of 0", allowed, known)
	}
}

// TestNeedsPromptOnlyWhereTheOSCanPrompt decides Enter's action: a media permission not yet asked, or accessibility
// the user denied.
func TestNeedsPromptOnlyWhereTheOSCanPrompt(t *testing.T) {
	cases := []struct {
		name string
		row  PermissionRow
		want bool
	}{
		{"microphone not asked", PermissionRow{Kind: platform.PermissionMicrophone, CanRequest: true, State: platform.PermissionNotDetermined}, true},
		{"microphone allowed", PermissionRow{Kind: platform.PermissionMicrophone, CanRequest: true, State: platform.PermissionAllowed}, false},
		{"camera denied opens settings", PermissionRow{Kind: platform.PermissionCamera, CanRequest: true, State: platform.PermissionDenied}, false},
		{"accessibility denied prompts", PermissionRow{Kind: platform.PermissionAccessibility, CanRequest: true, State: platform.PermissionDenied}, true},
		{"accessibility not asked opens settings", PermissionRow{Kind: platform.PermissionAccessibility, CanRequest: true, State: platform.PermissionNotDetermined}, false},
		{"screen recording has no prompt", PermissionRow{Kind: platform.PermissionScreenRecording, State: platform.PermissionNotDetermined}, false},
	}
	for _, tc := range cases {
		if got := tc.row.NeedsPrompt(); got != tc.want {
			t.Errorf("%s: NeedsPrompt = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestSubtitleSaysWhatEnterDoes names the action for each state.
func TestSubtitleSaysWhatEnterDoes(t *testing.T) {
	cases := []struct {
		row  PermissionRow
		want string
	}{
		{PermissionRow{Kind: platform.PermissionCamera, State: platform.PermissionAllowed}, "Allowed — press Enter to review in Settings"},
		{PermissionRow{Kind: platform.PermissionCamera, State: platform.PermissionDenied}, "Not allowed — press Enter to open Settings"},
		{PermissionRow{Kind: platform.PermissionMicrophone, State: platform.PermissionNotDetermined}, "Not asked yet — press Enter to allow"},
		{PermissionRow{Kind: platform.PermissionAccessibility, State: platform.PermissionUnknown}, "Could not check access — press Enter to open Settings"},
		{PermissionRow{Kind: platform.PermissionCamera, State: platform.PermissionUnknown}, "Review in Settings — press Enter to open"},
	}
	for _, tc := range cases {
		if got := tc.row.Subtitle(); got != tc.want {
			t.Errorf("Subtitle(%s, %s) = %q, want %q", tc.row.Kind, tc.row.State, got, tc.want)
		}
	}
}

// TestAllowedCountIgnoresUnknownRows counts only the rows the OS reported.
func TestAllowedCountIgnoresUnknownRows(t *testing.T) {
	states := fake.NewPermissions()
	states.SetState(platform.PermissionMicrophone, platform.PermissionAllowed)
	states.SetState(platform.PermissionCamera, platform.PermissionDenied)
	allowed, known := AllowedCount(PermissionRows("linux", states))
	if allowed != 1 || known != 2 {
		t.Fatalf("AllowedCount = %d of %d, want 1 of 2", allowed, known)
	}
}

// erroringPermissions fails every State call, as a broken OS query does, while claiming allowed.
type erroringPermissions struct{}

func (*erroringPermissions) State(platform.PermissionKind) (platform.PermissionState, error) {
	return platform.PermissionAllowed, errors.New("query failed")
}

func (*erroringPermissions) Request(platform.PermissionKind) error {
	return nil
}

// TestAPortErrorIsUnknownEvenWhenTheStateSaysAllowed reads the error, not the state, when the query fails.
func TestAPortErrorIsUnknownEvenWhenTheStateSaysAllowed(t *testing.T) {
	for _, row := range PermissionRows("linux", &erroringPermissions{}) {
		if row.State != platform.PermissionUnknown {
			t.Fatalf("%s = %q, want unknown after a query error", row.Kind, row.State)
		}
	}
}
