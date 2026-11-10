package app

import (
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// aliasStore returns the app's aliases, creating them on first use. Without a config store they live for the process.
func (p *App) aliasStore() *shortcuts.Aliases {
	if p.aliasList == nil {
		p.aliasList = shortcuts.NewAliases(p.aliasPersistence())
	}
	return p.aliasList
}

func (p *App) aliasPersistence() shortcuts.AliasPersistence {
	if p.sharedConfig == nil {
		return shortcuts.NewMemoryAliases()
	}
	return shortcuts.NewPortableAliases(p.sharedConfig)
}

// aliasMatchScore is how well the best of a row's aliases answers the query, 0 when none does.
func aliasMatchScore(aliases []string, query string) int {
	best := 0
	for _, alias := range aliases {
		best = max(best, search.Score(alias, query))
	}
	return best
}
