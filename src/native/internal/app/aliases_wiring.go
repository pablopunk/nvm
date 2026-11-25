package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/services"
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

// extensionAliasEditor changes the user's aliases for extensions and conversation programs. The root keeps snapshots of
// rows found by alias, so they are dropped after a change.
func (p *App) extensionAliasEditor() services.AliasEditor {
	table := shortcuts.NewExtensionTable(p.shortcutsManager(), p.labeler, p.aliasStore())
	return shortcuts.NewExtensionAliasEditor(table, p.rootAliasesChanged)
}

// rootAliasesChanged drops the root's remembered results and runs its rows again when it is on top. Safe from any
// goroutine.
func (p *App) rootAliasesChanged() {
	p.onUI(func() {
		if p.Palette != nil {
			p.ForgetCachedResults(isRootView)
		}
		p.RefreshTop(isRootView)
	})
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
