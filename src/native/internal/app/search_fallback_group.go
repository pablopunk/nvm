package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fallbackGroup publishes both built-in query fallbacks together, without holding other results.
type fallbackGroup struct {
	expected map[string]bool
	answered map[string]bool
	pending  map[string][]search.Item
}

func (g *fallbackGroup) targets(ids []string) {
	g.expected = map[string]bool{}
	for _, id := range ids {
		if id == aichats.ID || id == websearch.ID {
			g.expected[id] = true
		}
	}
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
	if (!g.expected[aichats.ID] || g.answered[aichats.ID]) && (!g.expected[websearch.ID] || g.answered[websearch.ID]) {
		g.flush(rows)
	}
}

func (g *fallbackGroup) flush(rows map[string][]search.Item) {
	for id, items := range g.pending {
		rows[id] = append(rows[id], items...)
	}
	clear(g.pending)
}
