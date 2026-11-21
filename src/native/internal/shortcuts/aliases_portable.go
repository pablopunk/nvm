package shortcuts

import (
	"maps"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/portable"
)

// portableAliases keeps the aliases in config.json under "aliases", keyed as 1.x keys them. Keys this build does not
// bind are read under their own name and written back unchanged.
type portableAliases struct {
	store *portable.Store
}

// NewPortableAliases returns the alias persistence over config.json.
func NewPortableAliases(store *portable.Store) AliasPersistence {
	return portableAliases{store: store}
}

// Load returns the aliases in config.json. An unreadable file returns no aliases with the reason.
func (p portableAliases) Load() (map[string][]string, error) {
	stored, err := p.store.Load()
	return aliasesOf(stored), err
}

// Update applies change to the aliases in config.json and writes them.
func (p portableAliases) Update(change func(map[string][]string)) error {
	return p.store.Update(func(stored portable.Document) {
		before := aliasesOf(stored)
		after := cloneAliases(before)
		change(after)
		writeAliases(stored, before, after)
	})
}

func aliasesOf(stored portable.Document) map[string][]string {
	all := map[string][]string{}
	for _, key := range stored.Keys("aliases") {
		value, _ := stored.Value("aliases", key)
		list, ok := value.([]any)
		if !ok {
			continue
		}
		var phrases []string
		for _, item := range list {
			if text, isText := item.(string); isText && strings.TrimSpace(text) != "" {
				phrases = append(phrases, text)
			}
		}
		if len(phrases) > 0 {
			all[aliasActionIDOf(key)] = phrases
		}
	}
	return all
}

// writeAliases makes the aliases in the file match after. A key whose aliases changed keeps its place in the file.
func writeAliases(stored portable.Document, before, after map[string][]string) {
	for _, id := range slices.Sorted(maps.Keys(before)) {
		if _, kept := after[id]; !kept {
			stored.Remove("aliases", aliasKeyOf(id))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(after)) {
		if slices.Equal(before[id], after[id]) {
			continue
		}
		list := make([]any, 0, len(after[id]))
		for _, phrase := range after[id] {
			list = append(list, phrase)
		}
		stored.SetValue(list, "aliases", aliasKeyOf(id))
	}
}

// aliasActionIDOf is the action ID of a config.json alias key: the stable ID of a 1.x key this build knows, else the key.
func aliasActionIDOf(key string) string {
	if actionID, bound := ActionIDOf(key); bound {
		return actionID
	}
	return key
}

// aliasKeyOf is the config.json key of an action's aliases.
func aliasKeyOf(actionID string) string {
	return PortableKeyOf(actionID)
}
