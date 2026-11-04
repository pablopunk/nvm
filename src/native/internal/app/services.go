package app

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	settingsfeat "github.com/pablopunk/nvm/native/internal/features/settings"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
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
	p.systemSvc = newSystemServices(ports)
	p.launchAtLogin = settingsfeat.NewLaunchAtLogin(ports.Autostart, ports.Capabilities.Supported(platform.CapabilityAutostart))
	var cache calc.RateCache
	// One store serves every persisted document of the app (rates, emoji recents, notes, snippets, learning, clipboard history).
	// The lease is taken once, here: a second instance reads the notes and snippets and persists nothing else.
	lease, err := store.AcquireLease(ports.FS, ports.Clock, dir, store.WithProcesses(ports.Processes))
	p.userReadOnly = err != nil
	if p.userReadOnly {
		log.Println("store: not persisting, the directory is held elsewhere:", err)
	}
	st, err := store.Open(ports.FS, ports.Clock, dir, store.WithDebounce(notes.DraftDelay), store.WithLease(lease))
	if err != nil {
		log.Println("store: open failed, not persisting:", err)
		if lease != nil {
			_ = lease.Release()
		}
	} else {
		p.userStore = st
		if !p.userReadOnly {
			p.store = st
			p.recents = store.Doc(st, "emoji-recents.json", 1, func() emojiRecents { return emojiRecents{} })
			p.rates = store.Doc(st, "rates.json", 1, defaultRateDocument)
			cache = rateCache{doc: p.rates}
		}
	}
	rates, err := calc.NewRateService(ports.HTTP, ports.Clock, cache)
	if err != nil {
		log.Println("calculator rates:", err)
	}
	p.calc = calc.New(ports.Clock, zone, rates)
}

// newSystemServices builds the OS capability services over the ports. The home folder is the process's, and the
// settings label is the one the OS uses.
func newSystemServices(ports platform.Ports) *systemfeat.Services {
	home, _ := os.UserHomeDir()
	return systemfeat.NewServices(systemfeat.Options{
		Ports:         ports,
		Platform:      runtime.GOOS,
		Home:          home,
		SettingsTitle: real.SettingsTitle(),
	})
}

// closeStoreOnQuit is kept for the call order in ready; the store is closed once, by openUserData, after the
// note windows have closed.
func (p *App) closeStoreOnQuit() {}

// prepareCatalog builds the emoji search indexes off the UI path, so the first keystroke does not pay for them.
func prepareCatalog() {
	safego.Go("emoji.prepare", func(context.Context) { emojiCatalog.Prepare() })
}
