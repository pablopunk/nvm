package permissions

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeSystem serves the permission calls from fixed statuses and records each request and pane.
type fakeSystem struct {
	api.System
	statuses   []api.PermissionStatus
	requestErr error
	paneErr    error
	requested  []string
	opened     []string
}

func (s *fakeSystem) Permissions() []api.PermissionStatus { return s.statuses }

func (s *fakeSystem) RequestPermission(kind string) error {
	s.requested = append(s.requested, kind)
	return s.requestErr
}

func (s *fakeSystem) OpenPermissionPane(kind string) error {
	s.opened = append(s.opened, kind)
	return s.paneErr
}

// fakeContext exposes only the System service.
type fakeContext struct {
	api.Context
	system *fakeSystem
}

func (c fakeContext) System() api.System { return c.system }

var permissionStatuses = []api.PermissionStatus{
	{Kind: api.PermissionMicrophone, Title: "Microphone", Icon: "mic", State: api.PermissionNotDetermined, CanRequest: true, Prompt: true},
	{Kind: api.PermissionCamera, Title: "Camera", Icon: "camera", State: api.PermissionAllowed, CanRequest: true},
	{Kind: api.PermissionAccessibility, Title: "Accessibility", Icon: "accessibility", State: api.PermissionUnknown, CanRequest: true},
}

func invoke(ref string, args map[string]string) api.Invocation {
	return api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}}
}

func openPage(t *testing.T, system *fakeSystem) api.View {
	t.Helper()
	result, err := Extension{}.RunAction(fakeContext{system: system}, invoke(openAction, nil))
	if err != nil || result.View == nil {
		t.Fatalf("open = %+v, %v; want a page", result, err)
	}
	return *result.View
}

func TestPageListsEveryPermissionWithItsState(t *testing.T) {
	view := openPage(t, &fakeSystem{statuses: permissionStatuses})
	if view.Kind != api.ViewList || view.Title != "Nevermind OS Permissions" || view.Placeholder != "Search permissions" {
		t.Fatalf("view = %+v, want the OS permissions list", view)
	}
	if len(view.Items) != len(permissionStatuses) {
		t.Fatalf("items = %d, want %d", len(view.Items), len(permissionStatuses))
	}
	if got := view.Items[0].Subtitle; got != "Not asked yet — press Enter to allow" {
		t.Errorf("microphone subtitle = %q", got)
	}
	if got := view.Items[1].Subtitle; got != "Allowed — press Enter to review in Settings" {
		t.Errorf("camera subtitle = %q", got)
	}
	if got := view.Items[2].Subtitle; got != "Could not check access — press Enter to open Settings" {
		t.Errorf("accessibility subtitle = %q", got)
	}
}

func TestPromptingPermissionAsksFirstAndOpensItsPaneSecond(t *testing.T) {
	item := openPage(t, &fakeSystem{statuses: permissionStatuses}).Items[0]
	if item.ID != "os-permission:microphone" {
		t.Errorf("id = %q", item.ID)
	}
	primary := item.Primary
	if primary == nil || primary.Title != "Allow Microphone" || primary.Ref == nil || primary.Ref.ID != refRequest || primary.Ref.Args[argKind] != api.PermissionMicrophone {
		t.Fatalf("primary = %+v, want the Allow Microphone request", primary)
	}
	if len(item.Actions) != 1 || item.Actions[0].Title != "Open Microphone Settings" || !item.Actions[0].DismissAfterRun {
		t.Fatalf("actions = %+v, want Open Microphone Settings, which closes the palette", item.Actions)
	}
}

func TestPermissionThatCannotPromptOpensItsPane(t *testing.T) {
	item := openPage(t, &fakeSystem{statuses: permissionStatuses}).Items[1]
	if item.Primary == nil || item.Primary.Title != "Open Camera Settings" || !item.Primary.DismissAfterRun {
		t.Fatalf("primary = %+v, want Open Camera Settings", item.Primary)
	}
	if len(item.Actions) != 0 {
		t.Fatalf("actions = %+v, want none", item.Actions)
	}
	if item.Primary.Ref == nil || item.Primary.Ref.ID != refOpenPane || item.Primary.Ref.Args[argKind] != api.PermissionCamera {
		t.Fatalf("ref = %+v, want the open-pane action for camera", item.Primary.Ref)
	}
}

func TestPageWithoutPermissionsSaysSo(t *testing.T) {
	view := openPage(t, &fakeSystem{})
	if view.EmptyView == nil || view.EmptyView.Title != "No permissions" {
		t.Fatalf("empty view = %+v, want No permissions", view.EmptyView)
	}
}

func TestRequestAsksTheOSAndShowsTheListAgain(t *testing.T) {
	system := &fakeSystem{statuses: permissionStatuses}
	result, err := Extension{}.RunAction(fakeContext{system: system}, invoke(refRequest, map[string]string{argKind: api.PermissionMicrophone}))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if len(system.requested) != 1 || system.requested[0] != api.PermissionMicrophone {
		t.Fatalf("requested = %v, want microphone", system.requested)
	}
	if result.View == nil || result.Navigation != api.NavReplace || result.Toast != nil {
		t.Fatalf("result = %+v, want the list replaced with no toast", result)
	}
}

func TestRefusedRequestSaysWhereToGo(t *testing.T) {
	const sentence = "Allow microphone in System Settings. Nevermind cannot show that prompt here."
	system := &fakeSystem{statuses: permissionStatuses, requestErr: errors.New(sentence)}
	result, err := Extension{}.RunAction(fakeContext{system: system}, invoke(refRequest, map[string]string{argKind: api.PermissionMicrophone}))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if result.Toast == nil || result.Toast.Message != sentence || result.Toast.Tone != "error" {
		t.Fatalf("toast = %+v, want the refusal as an error", result.Toast)
	}
	if result.View == nil {
		t.Fatal("a refused request still shows the list")
	}
}

func TestOpenPaneOpensTheKindsPane(t *testing.T) {
	system := &fakeSystem{statuses: permissionStatuses}
	result, err := Extension{}.RunAction(fakeContext{system: system}, invoke(refOpenPane, map[string]string{argKind: api.PermissionCamera}))
	if err != nil || result.Toast != nil {
		t.Fatalf("open pane = %+v, %v; want no toast", result, err)
	}
	if len(system.opened) != 1 || system.opened[0] != api.PermissionCamera {
		t.Fatalf("opened = %v, want camera", system.opened)
	}
}

func TestPaneFailureIsAToast(t *testing.T) {
	system := &fakeSystem{paneErr: errors.New("Could not open Camera Settings")}
	result, err := Extension{}.RunAction(fakeContext{system: system}, invoke(refOpenPane, map[string]string{argKind: api.PermissionCamera}))
	if err != nil || result.Toast == nil || result.Toast.Message != "Could not open Camera Settings" {
		t.Fatalf("result = %+v, %v; want the failure as a toast", result, err)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	if _, err := (Extension{}).RunAction(fakeContext{system: &fakeSystem{}}, invoke("bogus", nil)); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
}

func TestManifestDeclaresTheSystemCapability(t *testing.T) {
	manifest := Extension{}.Manifest()
	if manifest.Identity.ID != ID || len(manifest.Capabilities) != 1 || manifest.Capabilities[0] != api.CapabilitySystem {
		t.Fatalf("manifest = %+v, want the OS Permissions identity with the system declaration", manifest)
	}
}
