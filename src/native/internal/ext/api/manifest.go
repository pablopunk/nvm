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
	// Settings are the user-visible settings the extension declares. Values are stored per extension, and
	// the declaration is the only way a setting exists: Settings.Set on an undeclared id is refused.
	Settings []SettingDefinition `json:"settings,omitempty"`
	// CommandTitles renames commands by their ID. A command shows the title here when one is set, and the title its code
	// declares otherwise. The palette's Rename writes here, so renaming a command never edits the code. Only titles are
	// renamed: a subtitle stays as the code declares it.
	CommandTitles map[string]string `json:"commandTitles,omitempty"`
	// ConvertedFrom names the 1.x source file this folder was converted from, such as toggle-system-theme-257c802c.ts.
	// On its first scan the folder takes that file's enabled state from 1.x, so a converted extension that was on stays
	// on. It is host metadata: extension code never reads it.
	ConvertedFrom string `json:"convertedFrom,omitempty"`
}

// Capability names an area of intended behaviour shown in the extension review screen.
type Capability string

const (
	CapabilityDesktopApps         Capability = "desktop.apps"
	CapabilityDesktopFiles        Capability = "desktop.files"
	CapabilityDesktopClipboard    Capability = "desktop.clipboard"
	CapabilityDesktopSelection    Capability = "desktop.selection"
	CapabilityDesktopKeyboard     Capability = "desktop.keyboard"
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
	CapabilityNetwork             Capability = "network"
	CapabilitySecrets             Capability = "secrets"
	CapabilityNotes               Capability = "notes"
)

// KnownCapabilities lists every capability this host version can review.
func KnownCapabilities() []Capability {
	return []Capability{
		CapabilityDesktopApps, CapabilityDesktopFiles, CapabilityDesktopClipboard, CapabilityWindowManagement,
		CapabilityDesktopSelection, CapabilityDesktopKeyboard,
		CapabilityClipboardHistory, CapabilityAI, CapabilityExtensionsOwnership,
		CapabilityShortcuts, CapabilitySystem, CapabilityPlaces, CapabilityUpdates,
		CapabilitySettingsWrite, CapabilityDictation, CapabilityCamera, CapabilityOCR,
		CapabilityNetwork, CapabilitySecrets, CapabilityNotes,
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
// capability names; see UnknownCapabilities. Folder extensions may not use the reserved prefix.
func (m Manifest) Validate() error {
	if strings.HasPrefix(m.ID, reservedIDPrefix) {
		return fmt.Errorf("extension id %q uses the reserved %q prefix", m.ID, reservedIDPrefix)
	}
	return m.validateShape()
}

// ValidateBuiltIn checks a manifest compiled into Nevermind. It is Validate without the reserved prefix rule,
// which exists to keep folder extensions out of the built-in namespace.
func (m Manifest) ValidateBuiltIn() error {
	if !strings.HasPrefix(m.ID, reservedIDPrefix) {
		return fmt.Errorf("built-in extension id %q must use the %q prefix", m.ID, reservedIDPrefix)
	}
	return m.validateShape()
}

func (m Manifest) validateShape() error {
	if len(m.ID) > maxExtensionIDLength || !extensionIDPattern.MatchString(m.ID) {
		return fmt.Errorf("extension id %q must be lowercase dot-separated segments such as acme.world-clock", m.ID)
	}
	if strings.TrimSpace(m.Title) == "" {
		return errors.New("extension title is required")
	}
	if err := validateSettingDefinitions(m.Settings); err != nil {
		return err
	}
	for id, title := range m.CommandTitles {
		if strings.TrimSpace(id) == "" {
			return errors.New("commandTitles has an empty command id")
		}
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("commandTitles.%s must not be empty", id)
		}
	}
	if err := validateConvertedFrom(m.ConvertedFrom); err != nil {
		return err
	}
	return CheckCompatibility(m.APIVersion, Current)
}

// validateConvertedFrom accepts "" or the plain name of a 1.x extension source file: it ends in .ts and names no folder.
func validateConvertedFrom(name string) error {
	if name == "" {
		return nil
	}
	if !strings.HasSuffix(name, ".ts") || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("convertedFrom %q must be the file name of a 1.x extension, such as toggle-system-theme.ts", name)
	}
	return nil
}
