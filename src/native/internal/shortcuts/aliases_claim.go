package shortcuts

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// MaxAliasRunes is the longest alias: a phrase typed into the root search, not a sentence.
const MaxAliasRunes = 64

// CleanAlias is alias as it is saved, trimmed, or the reason it cannot be one.
func CleanAlias(alias string) (string, error) {
	cleaned := strings.TrimSpace(alias)
	switch {
	case cleaned == "":
		return "", fmt.Errorf("an alias needs at least one letter: %w", api.ErrInvalidInput)
	case utf8.RuneCountInString(cleaned) > MaxAliasRunes:
		return cleaned, fmt.Errorf("an alias has at most %d characters: %w", MaxAliasRunes, api.ErrInvalidInput)
	case strings.IndexFunc(cleaned, unicode.IsControl) >= 0:
		return cleaned, fmt.Errorf("an alias is one line of text: %w", api.ErrInvalidInput)
	}
	return cleaned, nil
}

// Owner returns the action that has alias, matched without regard to case.
func (a *Aliases) Owner(alias string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ownerIn(a.current(), alias)
}

// Claim saves alias for actionID unless an action already has it. Then it saves nothing and returns that action.
func (a *Aliases) Claim(actionID, alias string) (owner string, err error) {
	err = a.update(func(all map[string][]string) {
		if taken, found := ownerIn(all, alias); found {
			owner = taken
			return
		}
		all[actionID] = append(all[actionID], alias)
	})
	return owner, err
}

// Drop removes alias, matched without regard to case, from every action that has it.
func (a *Aliases) Drop(alias string) error {
	return a.update(func(all map[string][]string) {
		for actionID, list := range all {
			remaining := slices.DeleteFunc(slices.Clone(list), func(value string) bool { return strings.EqualFold(value, alias) })
			if len(remaining) == 0 {
				delete(all, actionID)
				continue
			}
			all[actionID] = remaining
		}
	})
}

// ownerIn is the first action, in ID order, whose aliases include alias.
func ownerIn(all map[string][]string, alias string) (string, bool) {
	ids := make([]string, 0, len(all))
	for actionID := range all {
		ids = append(ids, actionID)
	}
	slices.Sort(ids)
	for _, actionID := range ids {
		if slices.ContainsFunc(all[actionID], func(value string) bool { return strings.EqualFold(value, alias) }) {
			return actionID, true
		}
	}
	return "", false
}
