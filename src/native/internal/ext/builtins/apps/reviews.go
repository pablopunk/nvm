package apps

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// maxKeptReviews bounds the uninstall reviews the built-in keeps. The oldest is dropped first.
const maxKeptReviews = 8

// review is one uninstall review the palette shows: the app, the candidates the host listed and the selection the user
// made so far.
type review struct {
	name     string
	items    []api.UninstallItem
	notes    []string
	selected map[string]bool
}

// reviewBook keeps the reviews by the token the host gave them, so a toggle changes the selection the review holds.
type reviewBook struct {
	mu      sync.Mutex
	byToken map[string]*review
	order   []string
}

func newReviewBook() *reviewBook {
	return &reviewBook{byToken: map[string]*review{}}
}

// keep stores a review under its token.
func (b *reviewBook) keep(token string, r *review) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, known := b.byToken[token]; !known {
		b.order = append(b.order, token)
	}
	b.byToken[token] = r
	for len(b.order) > maxKeptReviews {
		delete(b.byToken, b.order[0])
		b.order = b.order[1:]
	}
}

// lookup returns the review of token, and false when the book does not hold it.
func (b *reviewBook) lookup(token string) (*review, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.byToken[token]
	return r, ok
}

// drop forgets a review once it has run.
func (b *reviewBook) drop(token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.byToken, token)
	for i, kept := range b.order {
		if kept == token {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
}

// toggle flips the selection of one item of a review and returns the review.
func (b *reviewBook) toggle(token, id string) (*review, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.byToken[token]
	if !ok {
		return nil, false
	}
	r.selected[id] = !r.selected[id]
	return r, true
}

// chosen returns the candidates the review has selected, in the order the host listed them.
func (r *review) chosen() []api.UninstallItem {
	var out []api.UninstallItem
	for _, item := range r.items {
		if r.selected[item.ID] {
			out = append(out, item)
		}
	}
	return out
}
