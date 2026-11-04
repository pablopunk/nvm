package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// serveSystem answers read-only host metadata. It needs no declaration: it reveals nothing an extension could misuse.
func (h *Handler) serveSystem(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpOS:
		return h.deps.OS, nil
	case protocol.OpHasCapability:
		return h.hasCapability(call.Args)
	case protocol.OpLabels:
		return systemLabelsOf(platform.ShellLabelsFor(h.deps.OS)), nil
	case protocol.OpSettingsPanes:
		return h.settingsPanes(), nil
	case protocol.OpPlaces:
		return h.places(), nil
	case protocol.OpNotify:
		return nil, h.notify(call.Args)
	}
	return nil, invalid("system has no operation %q", call.Op)
}

func (h *Handler) hasCapability(raw json.RawMessage) (bool, error) {
	var args protocol.CapabilityArgs
	if err := decodeArgs(raw, &args); err != nil {
		return false, err
	}
	return h.publicCapability(args.Name), nil
}

// publicCapability maps the names extensions check to host capabilities. An unknown name is false, so an extension
// that asks about a feature this host lacks omits it.
func (h *Handler) publicCapability(name string) bool {
	switch name {
	case "ocr":
		return ocr.Availability(h.deps.Capabilities).Image
	case "ocr.screen":
		return ocr.Availability(h.deps.Capabilities).Screen
	case "window-management":
		return h.deps.Capabilities.Supported(platform.CapabilityWindowManagement)
	case "apps.frontmost", "notifications.show",
		"shell.reveal", "shell.installedApps", "shell.appIcons", "clipboard.image", "clipboard.changed", "clipboard.paste", "selection.copy",
		"keyboard.type-text", "apps.running", "apps.quit", "apps.hide", "apps.uninstall",
		"system.lock", "system.sleep", "system.restart", "system.shutDown", "system.logOut", "system.emptyTrash", "system.settings":
		return h.deps.Capabilities.Supported(platform.Capability(name))
	case "clipboard.concealed":
		_, ok := h.deps.Clipboard.(platform.ConcealedClipboard)
		return ok
	}
	return false
}

// settingsPanes lists the panes the host can open. Without a provider the OS has none to offer.
func (h *Handler) settingsPanes() []api.SettingsPane {
	if h.deps.SettingsPanes == nil {
		return nil
	}
	return h.deps.SettingsPanes()
}

// places lists the standard folders of the home folder. Without a provider the host has none to offer.
func (h *Handler) places() []api.Place {
	if h.deps.Places == nil {
		return nil
	}
	return h.deps.Places()
}

func systemLabelsOf(labels platform.ShellLabels) api.SystemLabels {
	return api.SystemLabels{
		RevealInFileManager: labels.RevealInFileManager,
		PreviewFile:         labels.PreviewFile,
		OpenSystemSettings:  labels.OpenSystemSettings,
		KeyboardSettings:    labels.KeyboardSettings,
		SettingsCommand:     labels.SettingsCommand,
	}
}
