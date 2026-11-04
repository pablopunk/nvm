package system

// CommandID names one OS command the host runs. The built-in system commands extension names them by op, and the
// ops are stable across releases.
type CommandID string

// The OS commands the host runs.
const (
	CommandLockScreen  CommandID = "lock-screen"
	CommandSleep       CommandID = "sleep"
	CommandRestart     CommandID = "restart"
	CommandShutDown    CommandID = "shut-down"
	CommandLogOut      CommandID = "log-out"
	CommandEmptyTrash  CommandID = "empty-trash"
	CommandSystemPanes CommandID = "open-system-settings"
)

// CommandForOp maps the op an extension names to the command the host runs. An op the host does not run has none.
func CommandForOp(op string) (CommandID, bool) {
	id := CommandID(op)
	switch id {
	case CommandLockScreen, CommandSleep, CommandRestart, CommandShutDown, CommandLogOut, CommandEmptyTrash, CommandSystemPanes:
		return id, true
	}
	return "", false
}

// Title names a command in the sentence that says why it did not run. The settings command takes the OS label the
// composition root passes in.
func Title(id CommandID, settingsTitle string) string {
	switch id {
	case CommandLockScreen:
		return "Lock Screen"
	case CommandSleep:
		return "Sleep"
	case CommandRestart:
		return "Restart"
	case CommandShutDown:
		return "Shut Down"
	case CommandLogOut:
		return "Log Out"
	case CommandEmptyTrash:
		return "Empty Trash"
	case CommandSystemPanes:
		return settingsTitle
	}
	return string(id)
}
