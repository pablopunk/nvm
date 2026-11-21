package system

import "github.com/pablopunk/nvm/native/internal/platform"

type commandBinding struct {
	capability platform.Capability
	run        func() error
}

// Supported reports structural support without running a command or probing permission.
func (s *Services) Supported(id CommandID) bool {
	if s == nil {
		return false
	}
	binding := s.commandBinding(id)
	return binding.run != nil && s.ports.Capabilities.Supported(binding.capability)
}

// SupportsCapability preserves the public aliases using the same command binding as execution.
func (s *Services) SupportsCapability(capability platform.Capability) bool {
	if s == nil {
		return false
	}
	for _, id := range []CommandID{CommandLockScreen, CommandSleep, CommandRestart, CommandShutDown, CommandLogOut, CommandEmptyTrash, CommandSystemPanes} {
		if s.commandBinding(id).capability == capability {
			return s.Supported(id)
		}
	}
	return false
}

func (s *Services) commandBinding(id CommandID) commandBinding {
	system := s.ports.System
	if system == nil {
		return commandBinding{}
	}
	switch id {
	case CommandLockScreen:
		return commandBinding{platform.CapabilitySystemLock, system.LockScreen}
	case CommandSleep:
		return commandBinding{platform.CapabilitySystemSleep, system.Sleep}
	case CommandRestart:
		return commandBinding{platform.CapabilitySystemRestart, system.Restart}
	case CommandShutDown:
		return commandBinding{platform.CapabilitySystemShutDown, system.ShutDown}
	case CommandLogOut:
		return commandBinding{platform.CapabilitySystemLogOut, system.LogOut}
	case CommandEmptyTrash:
		return commandBinding{platform.CapabilitySystemEmptyTrash, system.EmptyTrash}
	case CommandSystemPanes:
		return commandBinding{platform.CapabilitySystemSettings, func() error { return system.OpenSettings("", "") }}
	}
	return commandBinding{}
}
