package manager

import (
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/legacy"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// inheritLocked gives each folder that names a 1.x source file the enabled state that file had. It runs on the first
// scan of such a folder only: the state is recorded, so a later change made here stays. A 1.x state that cannot be read
// records nothing, so the next scan tries again.
func (m *Manager) inheritLocked(bundles []bundle.Bundle) {
	if m.legacyStatePath == "" {
		return
	}
	var legacyOn map[string]bool
	loaded := false
	for _, b := range bundles {
		id, from := b.Manifest.ID, b.Manifest.ConvertedFrom
		if from == "" || m.inheritanceRecorded(id) {
			continue
		}
		if !loaded {
			loaded = true
			enabled, found, err := legacy.EnabledFiles(m.ports.Files, m.legacyStatePath)
			if err != nil {
				m.warnInheritance(err.Error())
				return
			}
			if !found {
				return
			}
			legacyOn = enabled
		}
		if err := m.enabled.Inherit(id, from, legacyOn[from]); err != nil {
			m.warnInheritance("save the inherited state of " + id + ": " + err.Error())
		}
	}
}

func (m *Manager) inheritanceRecorded(id string) bool {
	done, err := m.enabled.Inherited(id)
	return err == nil && done
}

func (m *Manager) warnInheritance(reason string) {
	if m.ports.Logs != nil {
		m.ports.Logs.Warn("extension state was not inherited from 1.x", logx.F{"reason": reason})
	}
}
