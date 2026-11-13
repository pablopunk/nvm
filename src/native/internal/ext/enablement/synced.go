package enablement

import (
	"sort"
	"strings"

	"github.com/pablopunk/nvm/native/internal/portable"
)

// configKey is the config.json section that holds the extension choices. 1.x keeps sections it does not know.
const configKey = "extensions"

// Pin is the review a choice is bound to: the declared capabilities, sorted and without repeats.
func Pin(capabilities []string) string {
	unique := map[string]bool{}
	for _, c := range capabilities {
		unique[c] = true
	}
	sorted := make([]string, 0, len(unique))
	for c := range unique {
		sorted = append(sorted, c)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// covers reports whether every capability in current was in the reviewed pin.
func covers(reviewed string, current []string) bool {
	known := map[string]bool{}
	for _, c := range strings.Split(reviewed, ",") {
		known[c] = true
	}
	for _, c := range current {
		if !known[c] {
			return false
		}
	}
	return true
}

// syncedEnabled answers from config.json. A local choice nobody has moved yet is moved first, pinned to the
// capabilities declared now, since that is what was running when the user chose it.
func (s *Set) syncedEnabled(state State, id string, capabilities []string) (bool, error) {
	cfg, err := s.synced.Load()
	if err != nil {
		return false, err
	}
	if reviewed, found := cfg.String(configKey, "enabled", id, "capabilities"); found {
		return covers(reviewed, capabilities), nil
	}
	if !state.Enabled[id] || state.Migrated[id] {
		return false, nil
	}
	if err := s.SetEnabled(id, true, capabilities); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Set) writeSynced(id string, enabled bool, capabilities []string) error {
	return s.synced.Update(func(doc portable.Document) {
		if enabled {
			doc.SetValue(Pin(capabilities), configKey, "enabled", id, "capabilities")
			return
		}
		doc.Remove(configKey, "enabled", id)
	})
}

func (s *Set) inheritSynced(id, convertedFrom string, on bool, capabilities []string) error {
	return s.synced.Update(func(doc portable.Document) {
		doc.SetValue(convertedFrom, configKey, "inherited", id)
		if _, already := doc.Value(configKey, "enabled", id); on && !already {
			doc.SetValue(Pin(capabilities), configKey, "enabled", id, "capabilities")
		}
	})
}
