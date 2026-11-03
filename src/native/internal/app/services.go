package app

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// emojiCatalog is the built-in character catalog. It holds no state, so every view shares one value.
var emojiCatalog = emoji.NewCatalog()

// rateDocument is rates.json: the exchange-rate quotes the calculator has fetched, keyed "BASE/QUOTE".
type rateDocument struct {
	Rates calc.Rates
}

func defaultRateDocument() rateDocument {
	return rateDocument{Rates: calc.Rates{}}
}

// rateCache adapts the rates document to the calculator's cache port.
type rateCache struct {
	doc *store.Collection[rateDocument]
}

// Load returns the saved quotes.
func (c rateCache) Load() (calc.Rates, error) {
	v, err := c.doc.Load()
	return v.Rates, err
}

// Save replaces the saved quotes.
func (c rateCache) Save(rates calc.Rates) error {
	return c.doc.Update(func(v *rateDocument) { v.Rates = rates })
}

// attachServices gives the app its ports and its persisted documents, and builds the calculator over them. The
// documents live in dir; zone is where calendar answers are computed (nil means the machine's zone). When this
// instance cannot take the directory's lease, nothing is persisted and the features run from memory: a second
// writer could overwrite the first.
func (p *App) attachServices(ports platform.Ports, dir string, zone *time.Location) {
	p.ports = ports
	var cache calc.RateCache
	lease, err := store.AcquireLease(ports.FS, ports.Clock, dir)
	if err != nil {
		log.Println("store: not persisting, the directory is held elsewhere:", err)
	} else if st, err := store.Open(ports.FS, ports.Clock, dir, store.WithLease(lease)); err != nil {
		log.Println("store: open failed, not persisting:", err)
	} else {
		p.store = st
		p.recents = store.Doc(st, "emoji-recents.json", 1, func() emojiRecents { return emojiRecents{} })
		p.rates = store.Doc(st, "rates.json", 1, defaultRateDocument)
		cache = rateCache{doc: p.rates}
	}
	rates, err := calc.NewRateService(ports.HTTP, ports.Clock, cache)
	if err != nil {
		log.Println("calculator rates:", err)
	}
	p.calc = calc.New(ports.Clock, zone, rates)
}

// closeStoreOnQuit flushes the documents and releases the lease when the app quits.
func (p *App) closeStoreOnQuit() {
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if p.store != nil {
			if err := p.store.Close(); err != nil {
				log.Println("store close:", err)
			}
		}
	})
}

// prepareCatalog builds the emoji search indexes off the UI path, so the first keystroke does not pay for them.
func prepareCatalog() {
	safego.Go("emoji.prepare", func(context.Context) { emojiCatalog.Prepare() })
}
