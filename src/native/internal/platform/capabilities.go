package platform

import "slices"

// Capability names an intent the current system may or may not fulfill. Features ask Supported before
// showing a row; the names describe the outcome, not the mechanism.
type Capability string

// Every capability the ports can report. Add a constant here when a port gains an intent that some OS
// lacks, and document the OS behavior on the port.
const (
	CapabilityClipboardImage   Capability = "clipboard.image"
	CapabilityClipboardChanged Capability = "clipboard.changed"
	CapabilityClipboardPaste   Capability = "clipboard.paste"
	CapabilityShellReveal      Capability = "shell.reveal"
	CapabilityShellTrash       Capability = "shell.trash"
	CapabilityShellApps        Capability = "shell.installedApps"
	CapabilityShellAppIcons    Capability = "shell.appIcons"
	CapabilityShellBeep        Capability = "shell.beep"
	CapabilityScreensDisplays  Capability = "screens.displays"
	CapabilityNotifications    Capability = "notifications.show"
	CapabilityKeychain         Capability = "keychain.store"
	CapabilityProcessList      Capability = "processes.list"
	CapabilityProcessSignal    Capability = "processes.signal"
	CapabilityHelperRun        Capability = "helper.run"
	CapabilityGlobalHotkeys    Capability = "hotkeys.global"
	CapabilityTrayMenu         Capability = "tray.menu"
	CapabilityAutostart        Capability = "autostart.signIn"
	CapabilityFileWatch        Capability = "files.watch"
	CapabilitySystemLock       Capability = "system.lock"
	CapabilitySystemSleep      Capability = "system.sleep"
	CapabilitySystemRestart    Capability = "system.restart"
	CapabilitySystemShutDown   Capability = "system.shutDown"
	CapabilitySystemLogOut     Capability = "system.logOut"
	CapabilitySystemEmptyTrash Capability = "system.emptyTrash"
	CapabilitySystemSettings   Capability = "system.settings"
)

// AllCapabilities lists every capability, sorted by name.
func AllCapabilities() []Capability {
	list := []Capability{
		CapabilityAutostart, CapabilityClipboardChanged, CapabilityClipboardImage, CapabilityClipboardPaste,
		CapabilityFileWatch, CapabilityGlobalHotkeys, CapabilityHelperRun, CapabilityKeychain,
		CapabilityNotifications, CapabilityProcessList, CapabilityProcessSignal, CapabilityScreensDisplays,
		CapabilityShellApps, CapabilityShellAppIcons, CapabilityShellBeep, CapabilityShellReveal,
		CapabilityShellTrash, CapabilitySystemEmptyTrash, CapabilitySystemLock, CapabilitySystemLogOut,
		CapabilitySystemRestart, CapabilitySystemSettings, CapabilitySystemShutDown, CapabilitySystemSleep,
		CapabilityTrayMenu,
	}
	slices.Sort(list)
	return list
}

// Capabilities is the set of capabilities one system supports. The zero value supports nothing.
type Capabilities struct {
	supported map[Capability]bool
}

// NewCapabilities returns the set that supports exactly the listed capabilities.
func NewCapabilities(list ...Capability) Capabilities {
	set := Capabilities{supported: make(map[Capability]bool, len(list))}
	for _, name := range list {
		set.supported[name] = true
	}
	return set
}

// Supported reports whether the system supports the capability. Unknown names are not supported.
func (c Capabilities) Supported(name Capability) bool {
	return c.supported[name]
}

// List returns the supported capabilities, sorted by name.
func (c Capabilities) List() []Capability {
	list := make([]Capability, 0, len(c.supported))
	for name, ok := range c.supported {
		if ok {
			list = append(list, name)
		}
	}
	slices.Sort(list)
	return list
}
