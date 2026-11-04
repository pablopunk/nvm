package app

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// nativeClientVersion is the desktop version the Go app reports to the backend's compatibility checks.
const nativeClientVersion = "2.0.0-go"

// isPackagedBuild is true for every build this app ships. Development backends stay out of reach until the
// release build flags exist (WP5.5).
const isPackagedBuild = true

// attachAccount opens the account session over st. Without a store the session keeps its profile in memory, so
// the sign-in still works for this run.
func (p *App) attachAccount(ports platform.Ports, st *store.Store) {
	var profile *store.Collection[auth.Profile]
	if st != nil {
		profile = auth.ProfileDocument(st)
	}
	session, err := auth.New(p.accountDeps(ports, profile))
	if err != nil {
		log.Println("account: not connected to a backend:", err)
		return
	}
	p.account = session
}

// accountDeps is the session wiring of this build. Only automation builds change it, to point the account at the
// rig's fake backend (account_rig_on.go).
func (p *App) accountDeps(ports platform.Ports, profile *store.Collection[auth.Profile]) auth.Deps {
	deps := auth.Deps{
		HTTP:          ports.HTTP,
		Clock:         ports.Clock,
		Keychain:      ports.Keychain,
		Logger:        logx.Default(),
		Profile:       profile,
		Browser:       ports.Shell.OpenURL,
		Packaged:      isPackagedBuild,
		ClientVersion: nativeClientVersion,
		DeviceLabel:   deviceLabel(),
		OnChange:      p.refreshAccountPage,
	}
	applyRigBackend(&deps)
	return deps
}

// attachDeepLinks claims the sign-in scheme so that nvm:// links from the approval page reach the session.
func (p *App) attachDeepLinks() {
	if p.account == nil || p.ports.DeepLinks == nil {
		return
	}
	if _, err := p.account.Attach(p.ports.DeepLinks); err != nil {
		log.Println("deep links: the sign-in scheme was not registered:", err)
	}
}

// deviceLabel names this computer on the backend's sign-in page, as the desktop app does.
func deviceLabel() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "this device"
	}
	return fmt.Sprintf("%s (%s)", host, runtime.GOOS)
}

// refreshAccountPage redraws the palette after an account change. Flows report from their own goroutines, so the
// redraw is queued on the UI thread.
func (p *App) refreshAccountPage() {
	p.onUI(func() {
		p.Refresh()
		if p.chats != nil {
			p.chats.AccountChanged()
		}
	})
}

// onUI runs fn on the UI thread. Without a window, which only the goldens and tests have, fn runs directly.
func (p *App) onUI(fn func()) {
	if p.panel != nil {
		p.panel.Update(fn)
		return
	}
	fn()
}

// signedIn reports whether the account is connected, from the snapshot, without reading the keychain.
func (p *App) signedIn() bool {
	return p.accountSource().Connected
}

// accountSummary is the settings row's accessory: who is signed in, or that nobody is.
func (p *App) accountSummary() string {
	s := p.accountSource()
	if !s.Connected {
		return "Not signed in"
	}
	return "Signed in as " + s.Email
}
