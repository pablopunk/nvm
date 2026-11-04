package protocol

// Host services of the desktop, clipboard history, OCR, system and shortcut APIs. Each name matches the capability
// that gates it, except system, which only reports host metadata and needs no declaration.
const (
	ServiceDesktopClipboard = "desktop.clipboard"
	ServiceDesktopApps      = "desktop.apps"
	ServiceDesktopFiles     = "desktop.files"
	ServiceDesktopShell     = "desktop.shell"
	ServiceDesktopWindows   = "desktop.windows"
	ServiceClipboardHistory = "clipboard.history"
	ServiceOCR              = "ocr"
	ServiceSystem           = "system"
	ServiceShortcuts        = "shortcuts"
)

// Operations of the desktop-side services. The names are the host's own; a child never sends a method name.
const (
	OpReadText      = "read-text"
	OpWriteText     = "write-text"
	OpList          = "list"
	OpSearch        = "search"
	OpLaunch        = "launch"
	OpIcon          = "icon"
	OpOpen          = "open"
	OpReveal        = "reveal"
	OpOpenExternal  = "open-external"
	OpDisplays      = "displays"
	OpWindows       = "windows"
	OpFocus         = "focus"
	OpApplyTo       = "apply-to"
	OpRemove        = "remove"
	OpHasCapability = "has-capability"
	OpOS            = "os"
	OpLabels        = "labels"
	OpSettingsPanes = "settings-panes"
	OpPalette       = "palette"
	OpImage         = "image"
	OpScreen        = "screen"
	OpRegion        = "region"
)
