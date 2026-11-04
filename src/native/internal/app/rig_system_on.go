//go:build automation

package app

import (
	"fmt"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// rigCallLogEnv names the file the rig reads to learn which system commands the app asked for.
const rigCallLogEnv = "NVM_RIG_SYSTEM_LOG"

// rigSystemCapabilities are the system.* intents the recorder answers for. rigSystem records each of them, so the rig
// offers those commands on any image, whatever programs the image has installed.
var rigSystemCapabilities = []platform.Capability{
	platform.CapabilitySystemLock,
	platform.CapabilitySystemSleep,
	platform.CapabilitySystemRestart,
	platform.CapabilitySystemShutDown,
	platform.CapabilitySystemLogOut,
	platform.CapabilitySystemEmptyTrash,
	platform.CapabilitySystemSettings,
}

// rigPorts gives an automation build a System port that runs nothing, and reports the capabilities that port records.
// The rig shares a desktop with other agents, so a confirmed restart, sleep or log out must only be recorded, never
// performed.
func rigPorts(ports platform.Ports) platform.Ports {
	ports.System = rigSystem{logPath: os.Getenv(rigCallLogEnv)}
	ports.Capabilities = platform.NewCapabilities(append(ports.Capabilities.List(), rigSystemCapabilities...)...)
	return ports
}

// rigSystem records each call as one line of the log file and performs nothing.
type rigSystem struct {
	logPath string
}

var _ platform.System = rigSystem{}

func (r rigSystem) LockScreen() error { return r.record("LockScreen") }
func (r rigSystem) Sleep() error      { return r.record("Sleep") }
func (r rigSystem) Restart() error    { return r.record("Restart") }
func (r rigSystem) ShutDown() error   { return r.record("ShutDown") }
func (r rigSystem) LogOut() error     { return r.record("LogOut") }
func (r rigSystem) EmptyTrash() error { return r.record("EmptyTrash") }

// OpenSettings is recorded with its pane and anchor, so a rig journey can tell which pane was asked for.
func (r rigSystem) OpenSettings(pane, anchor string) error {
	return r.record(fmt.Sprintf("OpenSettings %s %s", pane, anchor))
}

func (r rigSystem) record(call string) error {
	if r.logPath == "" {
		return nil
	}
	file, err := os.OpenFile(r.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, call)
	return err
}
