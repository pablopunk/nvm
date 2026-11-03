package api

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxExtensionIDLength = 128
	reservedIDPrefix     = "nevermind."
)

var extensionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z0-9][a-z0-9-]*)+$`)

// Identity is the stable identity of an extension. The ID never changes after publication:
// enablement, storage, secrets, global shortcuts and window restore keys are all keyed by it.
type Identity struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Manifest is the declarative header of an extension. The host reads it before any handler runs.
type Manifest struct {
	// APIVersion is the "major.minor" API version the extension was written against.
	APIVersion string `json:"apiVersion"`
	Identity
	Subtitle string `json:"subtitle,omitempty"`
	// Capabilities are review metadata: what the extension says it does. They are not an access
	// check. An enabled extension has the full host context whether or not it declares anything.
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// Capability names an area of intended behaviour shown in the extension review screen.
type Capability string

const (
	CapabilityDesktopApps         Capability = "desktop.apps"
	CapabilityDesktopFiles        Capability = "desktop.files"
	CapabilityWindowManagement    Capability = "desktop.window-management"
	CapabilityClipboardHistory    Capability = "clipboard.history"
	CapabilityAI                  Capability = "ai"
	CapabilityExtensionsOwnership Capability = "extensions.ownership"
	CapabilityShortcuts           Capability = "shortcuts"
	CapabilitySystem              Capability = "system"
	CapabilityPlaces              Capability = "places"
	CapabilityUpdates             Capability = "updates"
	CapabilitySettingsWrite       Capability = "settings.write"
	CapabilityDictation           Capability = "dictation"
	CapabilityCamera              Capability = "camera"
	CapabilityOCR                 Capability = "ocr"
)

// KnownCapabilities lists every capability this host version can review.
func KnownCapabilities() []Capability {
	return []Capability{
		CapabilityDesktopApps, CapabilityDesktopFiles, CapabilityWindowManagement,
		CapabilityClipboardHistory, CapabilityAI, CapabilityExtensionsOwnership,
		CapabilityShortcuts, CapabilitySystem, CapabilityPlaces, CapabilityUpdates,
		CapabilitySettingsWrite, CapabilityDictation, CapabilityCamera, CapabilityOCR,
	}
}

// UnknownCapabilities returns declared capabilities this host does not know. They are shown as
// diagnostics and never block loading.
func (m Manifest) UnknownCapabilities() []Capability {
	known := map[Capability]bool{}
	for _, capability := range KnownCapabilities() {
		known[capability] = true
	}
	var unknown []Capability
	for _, capability := range m.Capabilities {
		if !known[capability] {
			unknown = append(unknown, capability)
		}
	}
	return unknown
}

// Validate checks the manifest shape and API compatibility. It does not reject unknown
// capability names; see UnknownCapabilities.
func (m Manifest) Validate() error {
	if len(m.ID) > maxExtensionIDLength || !extensionIDPattern.MatchString(m.ID) {
		return fmt.Errorf("extension id %q must be lowercase dot-separated segments such as acme.world-clock", m.ID)
	}
	if strings.HasPrefix(m.ID, reservedIDPrefix) {
		return fmt.Errorf("extension id %q uses the reserved %q prefix", m.ID, reservedIDPrefix)
	}
	if strings.TrimSpace(m.Title) == "" {
		return errors.New("extension title is required")
	}
	return CheckCompatibility(m.APIVersion, Current)
}
