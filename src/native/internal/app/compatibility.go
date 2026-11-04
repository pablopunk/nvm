package app

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/store"
)

// compatibilityFile is the store document of the backend compatibility the menu bar reads. It keeps each backend's
// answer, so a restart offline still shows the update row, as the 1.x cache does.
const compatibilityFile = "compatibility.json"

// compatibilityDocument maps each backend's base URL to whether its manifest accepts this client.
type compatibilityDocument struct {
	Accepted map[string]bool `json:"accepted"`
}

// compatibility is the backend compatibility the menu bar reads, as 1.x reads its cache: the signed-in backend, and
// for each backend whether its manifest accepts this client. Only the account and the fetches change it.
type compatibility struct {
	mu       sync.Mutex
	doc      *store.Collection[compatibilityDocument]
	active   string
	accepted map[string]bool
	fetching bool
}

// restoreCompatibility opens the stored answers. A store that is not persisting leaves them empty for the run.
func (p *App) restoreCompatibility(st *store.Store) {
	if st == nil {
		return
	}
	p.compat.doc = store.Doc(st, compatibilityFile, 1, func() compatibilityDocument { return compatibilityDocument{} })
	stored, err := p.compat.doc.Load()
	if err != nil {
		return
	}
	p.compat.mu.Lock()
	defer p.compat.mu.Unlock()
	p.compat.accepted = map[string]bool{}
	for baseURL, accepted := range stored.Accepted {
		p.compat.accepted[baseURL] = accepted
	}
}

// setActive makes baseURL the signed-in backend, or none when baseURL is empty. It reports whether that changed.
func (c *compatibility) setActive(baseURL string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == baseURL {
		return false
	}
	c.active = baseURL
	return true
}

// rejectsClient reports whether the signed-in backend's manifest refuses this client. An unknown backend is not refused.
func (c *compatibility) rejectsClient() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	accepted, known := c.accepted[c.active]
	return c.active != "" && known && !accepted
}

// record stores whether baseURL accepts this client, and reports whether the answer changed.
func (c *compatibility) record(baseURL string, accepted bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accepted == nil {
		c.accepted = map[string]bool{}
	}
	previous, known := c.accepted[baseURL]
	c.accepted[baseURL] = accepted
	return !known || previous != accepted
}

// beginFetch reserves the single fetch in flight. It reports false when one is already running.
func (c *compatibility) beginFetch() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fetching {
		return false
	}
	c.fetching = true
	return true
}

func (c *compatibility) endFetch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetching = false
}

// persist writes the answers to the store document, when there is one.
func (c *compatibility) persist(baseURL string, accepted bool) {
	if c.doc == nil {
		return
	}
	_ = c.doc.Update(func(doc *compatibilityDocument) {
		if doc.Accepted == nil {
			doc.Accepted = map[string]bool{}
		}
		doc.Accepted[baseURL] = accepted
	})
}
