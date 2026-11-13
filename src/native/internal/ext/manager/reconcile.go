package manager

// ReconcileEnablement applies enabled choices that changed outside this process, such as a synced config.json that
// arrived from another machine: an extension that is now enabled starts, one that is now disabled or no longer covered
// by its reviewed capabilities stops. It reports whether any runtime changed.
func (m *Manager) ReconcileEnablement() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false
	}
	changed := false
	for key, e := range m.entries {
		if e.builtin || !e.bundle.Runnable() || m.isEnabled(e.bundle.Manifest) == e.enabledApplied {
			continue
		}
		m.replaceLocked(key, e.bundle)
		changed = true
	}
	if changed {
		m.editGeneration++
	}
	return changed
}
