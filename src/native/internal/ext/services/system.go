package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// serveSystem exposes host metadata and routes system effects through their declaration checks.
func (h *Handler) serveSystem(ctx context.Context, call protocol.ServiceCall) (any, error) {
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
		return nil, h.notify(ctx, call.Args)
	case protocol.OpPermissions:
		return h.permissionStatuses(), nil
	case protocol.OpRequestPermission:
		return nil, h.requestPermission(ctx, call.Args)
	case protocol.OpOpenPermissionPane:
		return nil, h.openPermissionPane(ctx, call.Args)
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
	case "network.download":
		_, ok := h.deps.Files.(platform.StreamFiles)
		return h.deps.Network != nil && ok
	case "network.fetch":
		return h.deps.Network != nil
	case "ocr":
		return h.deps.OCR.Availability(h.deps.Capabilities).Image
	case "ocr.screen":
		return h.deps.OCR.Availability(h.deps.Capabilities).Screen
	case "shell.exec":
		return platform.ProcessAvailability(h.deps.Commands, h.deps.OS).Exec
	case "shell.script":
		return platform.ProcessAvailability(h.deps.Commands, h.deps.OS).LoginScript
	case "quick-look":
		return h.filePreview().Supported()
	case "windows.switch-desktop":
		return h.deps.Windows != nil && h.deps.Desktops != nil && h.deps.Desktops.Supported()
	case "system.lock", "system.sleep", "system.restart", "system.shutDown", "system.logOut", "system.emptyTrash", "system.settings":
		return h.deps.SystemCommands.SupportsCapability(platform.Capability(name))
	case "apps.running", "apps.quit", "apps.hide", "apps.uninstall":
		return h.appLifecycleSupported(platform.Capability(name))
	case "window-management":
		return h.deps.Capabilities.Supported(platform.CapabilityWindowManagement)
	case "apps.frontmost", "notifications.show",
		"shell.reveal", "shell.installedApps", "shell.appIcons", "clipboard.image", "clipboard.changed", "clipboard.paste", "selection.copy",
		"keyboard.type-text", "autostart.signIn",
		"windows.always-on-top", "windows.all-spaces", "windows.frame-restore", "windows.display-recovery":
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
