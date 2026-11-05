package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteSystem reads host metadata. The api has no error results here, so a failed exchange reads as the zero value:
// no OS name, no capability and empty labels, which omit the rows that depend on them.
type remoteSystem struct {
	link *hostLink
}

func (s remoteSystem) OS() string {
	var name string
	_ = s.link.Call(protocol.ServiceSystem, protocol.OpOS, nil, &name)
	return name
}

func (s remoteSystem) HasCapability(name string) bool {
	var has bool
	_ = s.link.Call(protocol.ServiceSystem, protocol.OpHasCapability, protocol.CapabilityArgs{Name: name}, &has)
	return has
}

func (s remoteSystem) Labels() api.SystemLabels {
	var labels api.SystemLabels
	_ = s.link.Call(protocol.ServiceSystem, protocol.OpLabels, nil, &labels)
	return labels
}

func (s remoteSystem) SettingsPanes() []api.SettingsPane {
	var panes []api.SettingsPane
	_ = s.link.Call(protocol.ServiceSystem, protocol.OpSettingsPanes, nil, &panes)
	return panes
}

func (s remoteSystem) Places() []api.Place {
	var places []api.Place
	_ = s.link.Call(protocol.ServiceSystem, protocol.OpPlaces, nil, &places)
	return places
}

// Notify asks the host to show a notification. The host refuses it when the extension did not declare system.
func (s remoteSystem) Notify(title, body string) error {
	return s.link.Call(protocol.ServiceSystem, protocol.OpNotify, protocol.NotifyArgs{Title: title, Body: body}, nil)
}

// remoteShortcuts reads the shortcut table. A failed exchange reads as an empty table.
type remoteShortcuts struct {
	link *hostLink
}

func (s remoteShortcuts) List() []api.ShortcutRecord {
	var records []api.ShortcutRecord
	_ = s.link.Call(protocol.ServiceShortcuts, protocol.OpList, nil, &records)
	return records
}

func (s remoteShortcuts) Palette() api.ShortcutRecord {
	var record api.ShortcutRecord
	_ = s.link.Call(protocol.ServiceShortcuts, protocol.OpPalette, nil, &record)
	return record
}

// Aliases reads the aliases of the actions under prefix. A failed exchange reads as none.
func (s remoteShortcuts) Aliases(prefix string) map[string][]string {
	var aliases map[string][]string
	_ = s.link.Call(protocol.ServiceShortcuts, protocol.OpAliases, protocol.PrefixArgs{Prefix: prefix}, &aliases)
	return aliases
}

// remoteAccount reads whether the palette is signed in. A failed exchange reads as signed out.
type remoteAccount struct {
	link *hostLink
}

func (a remoteAccount) SignedIn() bool {
	var signedIn bool
	_ = a.link.Call(protocol.ServiceAccount, protocol.OpSignedIn, nil, &signedIn)
	return signedIn
}
