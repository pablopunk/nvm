package real

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// systemAction names one power or session program of the system port.
type systemAction string

const (
	actionLock       systemAction = "lock"
	actionSleep      systemAction = "sleep"
	actionRestart    systemAction = "restart"
	actionShutDown   systemAction = "shut-down"
	actionLogOut     systemAction = "log-out"
	actionEmptyTrash systemAction = "empty-trash"
)

// systemCommand is one program and its arguments. A chain of commands is tried in order until one works.
type systemCommand struct {
	name string
	args []string
}

// macLockCommand is the program that locks the macOS session.
const macLockCommand = "/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession"

// linuxSettingsChain opens the first desktop settings program the Linux desktop has.
var linuxSettingsChain = []systemCommand{
	{name: "gnome-control-center"},
	{name: "systemsettings"},
	{name: "xfce4-settings-manager"},
}

// systemChains is the command table: for each OS, the programs that carry out each action, in the order to try.
// An action absent from the table is not supported on that OS. Linux empties the Trash itself, so it has no
// entry for empty-trash.
func systemChains(goos, user string) map[systemAction][]systemCommand {
	switch goos {
	case "darwin":
		return map[systemAction][]systemCommand{
			actionLock:       {{name: macLockCommand, args: []string{"-suspend"}}},
			actionSleep:      {{name: "pmset", args: []string{"sleepnow"}}},
			actionRestart:    {appleScript(`tell application "System Events" to restart`)},
			actionShutDown:   {appleScript(`tell application "System Events" to shut down`)},
			actionLogOut:     {appleScript(`tell application "System Events" to log out`)},
			actionEmptyTrash: {appleScript(`tell application "Finder" to empty trash`)},
		}
	case "windows":
		return map[systemAction][]systemCommand{
			actionLock:     {{name: "rundll32.exe", args: []string{"user32.dll,LockWorkStation"}}},
			actionSleep:    {{name: "rundll32.exe", args: []string{"powrprof.dll,SetSuspendState", "0,1,0"}}},
			actionRestart:  {{name: "shutdown", args: []string{"/r", "/t", "0"}}},
			actionShutDown: {{name: "shutdown", args: []string{"/s", "/t", "0"}}},
			actionLogOut:   {{name: "shutdown", args: []string{"/l"}}},
		}
	case "linux":
		return linuxChains(user)
	}
	return map[systemAction][]systemCommand{}
}

// linuxChains is the systemd and freedesktop table. Log out needs the user's name; without one it is absent.
func linuxChains(user string) map[systemAction][]systemCommand {
	chains := map[systemAction][]systemCommand{
		actionLock: {
			{name: "loginctl", args: []string{"lock-session"}},
			{name: "xdg-screensaver", args: []string{"lock"}},
			{name: "gnome-screensaver-command", args: []string{"-l"}},
		},
		actionSleep:    {{name: "systemctl", args: []string{"suspend"}}},
		actionRestart:  {{name: "systemctl", args: []string{"reboot"}}},
		actionShutDown: {{name: "systemctl", args: []string{"poweroff"}}},
	}
	if user != "" {
		chains[actionLogOut] = []systemCommand{{name: "loginctl", args: []string{"terminate-user", user}}}
	}
	return chains
}

// appleScript is an osascript invocation of one script line.
func appleScript(line string) systemCommand {
	return systemCommand{name: "osascript", args: []string{"-e", line}}
}

// systemEnvironment is what the capability report depends on: the OS, the user and home folder, and which
// programs the system has. installed is an injection point; production asks the PATH.
type systemEnvironment struct {
	goos, user, home string
	installed        func(name string) bool
}

// processSystemEnvironment describes the running process.
func processSystemEnvironment() systemEnvironment {
	home, _ := os.UserHomeDir()
	return systemEnvironment{
		goos:      runtime.GOOS,
		user:      os.Getenv("USER"),
		home:      home,
		installed: programInstalled,
	}
}

// programInstalled reports whether the PATH, or the absolute path, names an executable.
func programInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// systemCapabilities reports which system.* capabilities the environment supports. Each one is supported when
// the command that carries it out is installed, so a Linux desktop without a screen locker does not offer Lock.
func systemCapabilities(env systemEnvironment) []platform.Capability {
	chains := systemChains(env.goos, env.user)
	actions := []struct {
		action     systemAction
		capability platform.Capability
	}{
		{actionLock, platform.CapabilitySystemLock},
		{actionSleep, platform.CapabilitySystemSleep},
		{actionRestart, platform.CapabilitySystemRestart},
		{actionShutDown, platform.CapabilitySystemShutDown},
		{actionLogOut, platform.CapabilitySystemLogOut},
	}
	var supported []platform.Capability
	for _, entry := range actions {
		if chainInstalled(chains[entry.action], env.installed) {
			supported = append(supported, entry.capability)
		}
	}
	if emptyTrashSupported(env, chains) {
		supported = append(supported, platform.CapabilitySystemEmptyTrash)
	}
	if settingsSupported(env) {
		supported = append(supported, platform.CapabilitySystemSettings)
	}
	return supported
}

// emptyTrashSupported is true on Linux when the home folder is known, and elsewhere when the OS has a command.
func emptyTrashSupported(env systemEnvironment, chains map[systemAction][]systemCommand) bool {
	if env.goos == "linux" {
		return env.home != ""
	}
	return chainInstalled(chains[actionEmptyTrash], env.installed)
}

// settingsSupported is true on macOS and Windows, which always have their settings app, and on Linux when a
// desktop settings program is installed.
func settingsSupported(env systemEnvironment) bool {
	switch env.goos {
	case "darwin", "windows":
		return true
	case "linux":
		return chainInstalled(linuxSettingsChain, env.installed)
	}
	return false
}

// chainInstalled reports whether the chain has at least one installed program.
func chainInstalled(chain []systemCommand, installed func(string) bool) bool {
	for _, command := range chain {
		if installed(command.name) {
			return true
		}
	}
	return false
}
