package app

import (
	"testing"

	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestDeniedAccessibilityOffersAPromptThenOpensSettings(t *testing.T) {
	p := goldenPalette(t)
	p.ports.Permissions = fake.NewPermissions()
	row := systemfeat.PermissionRow{Kind: platform.PermissionAccessibility, Title: "Accessibility",
		Pane: "com.apple.settings.PrivacySecurity.extension", Anchor: "Accessibility",
		State: platform.PermissionDenied, CanRequest: true}

	item := p.permissionItem(row)
	if len(item.Actions) != 2 || item.Actions[0].Title != "Allow Accessibility" || item.Actions[1].Title != "Open Accessibility Settings" {
		t.Fatalf("actions = %+v, want the prompt first and Open Accessibility Settings second", item.Actions)
	}
}

func TestUnknownAccessibilityOpensSettingsWithoutAPrompt(t *testing.T) {
	p := goldenPalette(t)
	row := systemfeat.PermissionRow{Kind: platform.PermissionAccessibility, Title: "Accessibility",
		Anchor: "Accessibility", State: platform.PermissionUnknown, CanRequest: true}

	item := p.permissionItem(row)
	if len(item.Actions) != 1 || item.Actions[0].Title != "Open Accessibility Settings" {
		t.Fatalf("actions = %+v, want only Open Accessibility Settings", item.Actions)
	}
}
