package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fallbackGroup publishes both built-in query fallbacks together, without holding other results.
type fallbackGroup struct {
	answered map[string]bool
	pending  map[string][]search.Item
}

func (g *fallbackGroup) accept(id string, items []search.Item, rows map[string][]search.Item) {
	if id != aichats.ID && id != websearch.ID {
		rows[id] = items
		return
	}
	if g.answered == nil {
		g.answered = map[string]bool{}
		g.pending = map[string][]search.Item{}
	}
	g.answered[id] = true
	for _, item := range items {
		if item.QueryFallback {
			g.pending[id] = append(g.pending[id], item)
		} else {
			rows[id] = append(rows[id], item)
		}
	}
	if g.answered[aichats.ID] && g.answered[websearch.ID] {
		g.flush(rows)
	}
}

func (g *fallbackGroup) flush(rows map[string][]search.Item) {
	for id, items := range g.pending {
		rows[id] = append(rows[id], items...)
	}
	clear(g.pending)
}
