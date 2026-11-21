package app

import (
	"testing"

	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestDeniedAccessibilityOffersAPromptThenOpensSettings(t *testing.T) {
	row := systemfeat.PermissionRow{Kind: platform.PermissionAccessibility, Title: "Accessibility",
		Pane: "com.apple.settings.PrivacySecurity.extension", Anchor: "Accessibility",
		State: platform.PermissionDenied, CanRequest: true}

	if status := apiPermissionStatus(row); !status.Prompt {
		t.Fatalf("status = %+v, want Enter to prompt first", status)
	}
}

func TestUnknownAccessibilityOpensSettingsWithoutAPrompt(t *testing.T) {
	row := systemfeat.PermissionRow{Kind: platform.PermissionAccessibility, Title: "Accessibility",
		Anchor: "Accessibility", State: platform.PermissionUnknown, CanRequest: true}

	if status := apiPermissionStatus(row); status.Prompt {
		t.Fatalf("status = %+v, want Enter to open Settings without a prompt", status)
	}
}
