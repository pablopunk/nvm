package app

import (
	"hash/fnv"
	"maps"
	"slices"
)

// rootDecorationBasis fingerprints the user state the root's rows are decorated from: the bound shortcuts a row shows
// and offers to remove, and the aliases that find it. Binding a shortcut or setting an alias changes it, so the root's
// fixed results are rebuilt once instead of showing the old decorations. It runs on the UI thread.
func (p *App) rootDecorationBasis() uint64 {
	h := fnv.New64a()
	write := func(parts ...string) {
		for _, part := range parts {
			_, _ = h.Write([]byte(part))
			_, _ = h.Write([]byte{0})
		}
	}
	bound := p.boundShortcuts()
	for _, id := range slices.Sorted(maps.Keys(bound)) {
		write("shortcut", id, bound[id])
	}
	aliases := p.aliasStore().All()
	for _, id := range slices.Sorted(maps.Keys(aliases)) {
		write(append([]string{"alias", id}, aliases[id]...)...)
	}
	return h.Sum64()
}
