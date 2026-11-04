package system

import "github.com/pablopunk/nvm/native/internal/platform"

// CommandID names one system command. Run dispatches on it, and it is stable across releases.
type CommandID string

// The system commands, in the order the palette lists them.
const (
	CommandLockScreen  CommandID = "lock-screen"
	CommandSleep       CommandID = "sleep"
	CommandRestart     CommandID = "restart"
	CommandShutDown    CommandID = "shut-down"
	CommandLogOut      CommandID = "log-out"
	CommandEmptyTrash  CommandID = "empty-trash"
	CommandSystemPanes CommandID = "system-settings"
)

// Command is one palette command the OS supports. Destructive commands are confirmed in the palette first; the
// Confirm text is the detail the confirmation shows.
type Command struct {
	ID          CommandID
	Title       string
	Subtitle    string
	Keywords    string
	Icon        string
	Tone        string
	Destructive bool
	Confirm     string
	capability  platform.Capability
}

// catalog is every system command with the capability that gates it. Titles and keywords are the 1.x wording;
// Trash and Log Out carry the Go prototype's wording, which is what users of the Go build already know.
func catalog(settingsTitle string) []Command {
	return []Command{
		{ID: CommandLockScreen, Title: "Lock Screen", Subtitle: "Secure this computer", Keywords: "lock screen security", Icon: "lock", capability: platform.CapabilitySystemLock},
		{ID: CommandSleep, Title: "Sleep", Subtitle: "Put this computer to sleep", Keywords: "sleep suspend", Icon: "moon", capability: platform.CapabilitySystemSleep},
		{ID: CommandLogOut, Title: "Log Out", Subtitle: "End the current session", Keywords: "logout sign out", Icon: "log-out", Destructive: true, Confirm: "End the current session. Unsaved work in open apps may be lost.", capability: platform.CapabilitySystemLogOut},
		{ID: CommandRestart, Title: "Restart", Subtitle: "Restart this computer", Keywords: "reboot restart", Icon: "rotate-ccw", Tone: "orange", Destructive: true, Confirm: "Reboot this computer. Unsaved work in open apps may be lost.", capability: platform.CapabilitySystemRestart},
		{ID: CommandShutDown, Title: "Shut Down", Subtitle: "Power off this computer", Keywords: "shutdown power off", Icon: "power", Tone: "red", Destructive: true, Confirm: "Power off this computer. Unsaved work in open apps may be lost.", capability: platform.CapabilitySystemShutDown},
		{ID: CommandEmptyTrash, Title: "Empty Trash", Subtitle: "Permanently delete trashed files", Keywords: "trash delete bin empty", Icon: "trash-2", Tone: "red", Destructive: true, Confirm: "Permanently delete the files in the Trash. This cannot be undone.", capability: platform.CapabilitySystemEmptyTrash},
		{ID: CommandSystemPanes, Title: settingsTitle, Subtitle: "Open system preferences", Keywords: "settings preferences system", Icon: "settings", capability: platform.CapabilitySystemSettings},
	}
}

// Commands returns the catalog entries the capabilities support, in catalog order.
func Commands(caps platform.Capabilities, settingsTitle string) []Command {
	var supported []Command
	for _, command := range catalog(settingsTitle) {
		if caps.Supported(command.capability) {
			supported = append(supported, command)
		}
	}
	return supported
}
