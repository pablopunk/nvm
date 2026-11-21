package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
)

// guardRootExtensionActions keeps cached declarative actions scoped to the installation that produced them.
// Built-ins already use host capability and resource checks; folder extensions can be disabled or replaced.
func guardRootExtensionActions(m *manager.Manager, id string, origin any, row search.Item) search.Item {
	if origin == nil && m.IsBuiltIn(id) {
		return row
	}
	for i := range row.Actions {
		run := row.Actions[i].Run
		if run == nil {
			continue
		}
		row.Actions[i].Run = func(h search.Host) {
			if origin == nil || m.SnapshotIdentity(id) != origin {
				h.Toast("This extension is no longer available.")
				return
			}
			run(h)
		}
	}
	return row
}
