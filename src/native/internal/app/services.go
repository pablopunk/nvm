package app

import (
	"log"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	settingsfeat "github.com/pablopunk/nvm/native/internal/features/settings"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/store"
)

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

// attachServices gives the app its ports and its persisted documents, and builds the calculator over them. goos is
// the operating system whose labels and panes the system features show. The documents live in dir; zone is where calendar answers are computed (nil means the machine's zone). When this
// instance cannot take the directory's lease, nothing is persisted and the features run from memory: a second
// writer could overwrite the first.
func (p *App) attachServices(ports platform.Ports, goos, dir string, zone *time.Location) {
	p.ports = ports
	p.goos = goos
	p.systemSvc = newSystemServices(ports, goos)
	p.launchAtLogin = settingsfeat.NewLaunchAtLogin(ports.Autostart, ports.Capabilities.Supported(platform.CapabilityAutostart))
	var cache calc.RateCache
	p.zone = zone
	// One store serves every persisted document of the app (rates, notes, snippets, learning, clipboard history).
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
			p.rates = store.Doc(st, "rates.json", 1, defaultRateDocument)
			p.restoreCompatibility(st)
			p.restoreMenuBarVisibility(st)
			cache = rateCache{doc: p.rates}
		}
	}
	p.sharedConfig = portable.Open(ports.FS, sharedConfigPath())
	p.attachAccount(ports, st)
	p.rateCache = cache
}

// newSystemServices builds the OS capability services over the ports for goos. The settings label is the one goos uses.
func newSystemServices(ports platform.Ports, goos string) *systemfeat.Services {
	return systemfeat.NewServices(systemfeat.Options{
		Ports:         ports,
		Platform:      goos,
		SettingsTitle: platform.ShellLabelsFor(goos).SettingsCommand,
	})
}
