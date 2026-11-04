package system

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// capsOf returns capabilities that support exactly the named ones.
func capsOf(names ...platform.Capability) platform.Capabilities {
	return platform.NewCapabilities(names...)
}

// commandIDs lists the IDs of commands in order.
func commandIDs(commands []Command) []CommandID {
	ids := make([]CommandID, 0, len(commands))
	for _, c := range commands {
		ids = append(ids, c.ID)
	}
	return ids
}

// TestCommandsAreOmittedWhenTheSystemLacksTheirCapability hides a command the OS cannot run rather than showing it.
func TestCommandsAreOmittedWhenTheSystemLacksTheirCapability(t *testing.T) {
	caps := capsOf(platform.CapabilitySystemSleep, platform.CapabilitySystemSettings)
	got := commandIDs(Commands(caps, "Open Settings"))
	want := []CommandID{CommandSleep, CommandSystemPanes}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands = %v, want %v", got, want)
	}
}

// TestCommandsKeepTheCatalogOrderWhenEverythingIsSupported lists every command in the order the palette shows.
func TestCommandsKeepTheCatalogOrderWhenEverythingIsSupported(t *testing.T) {
	all := capsOf(platform.AllCapabilities()...)
	got := commandIDs(Commands(all, "Open System Settings"))
	want := []CommandID{CommandLockScreen, CommandSleep, CommandLogOut, CommandRestart, CommandShutDown, CommandEmptyTrash, CommandSystemPanes}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands = %v, want %v", got, want)
	}
}

// TestDestructiveCommandsCarryAConfirmation makes sure no destructive command has an empty question to ask.
func TestDestructiveCommandsCarryAConfirmation(t *testing.T) {
	for _, c := range Commands(capsOf(platform.AllCapabilities()...), "Open Settings") {
		if c.Destructive && c.Confirm == "" {
			t.Errorf("%s is destructive without a confirmation", c.ID)
		}
		if !c.Destructive && c.Confirm != "" {
			t.Errorf("%s asks for confirmation but is not destructive", c.ID)
		}
	}
	for _, id := range []CommandID{CommandRestart, CommandShutDown, CommandLogOut, CommandEmptyTrash} {
		if !destructive(id) {
			t.Errorf("%s must be destructive", id)
		}
	}
}

// TestSettingsCommandUsesTheOSLabel titles the settings command with the label the composition root passes in.
func TestSettingsCommandUsesTheOSLabel(t *testing.T) {
	commands := Commands(capsOf(platform.CapabilitySystemSettings), "Open System Settings")
	if len(commands) != 1 || commands[0].Title != "Open System Settings" {
		t.Fatalf("Commands = %+v, want one settings command with the OS label", commands)
	}
}

// destructive looks a command up by ID and reports whether it is destructive.
func destructive(id CommandID) bool {
	for _, c := range catalog("") {
		if c.ID == id {
			return c.Destructive
		}
	}
	return false
}
