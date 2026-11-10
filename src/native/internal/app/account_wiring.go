package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// nativeClientVersion is the desktop version the Go app reports to the backend's compatibility checks.
const nativeClientVersion = "2.0.0-go"

// isPackagedBuild is "true" only in builds the packaging workflow links with -X (docs/packaging.md), so a development
// build keeps the development backends.
var isPackagedBuild = "false"

// packagedBuild reports whether this binary is a packaged build.
func packagedBuild() bool {
	return isPackagedBuild == "true"
}

// attachAccount opens the account session over st. Without a store the session keeps its profile in memory, so
// the sign-in still works for this run.
func (p *App) attachAccount(ports platform.Ports, st *store.Store) {
	var profile *store.Collection[auth.Profile]
	if st != nil {
		profile = auth.ProfileDocument(st)
		importLegacyAccount(ports.FS, profile)
	}
	session, err := auth.New(p.accountDeps(ports, profile))
	if err != nil {
		log.Println("account: not connected to a backend:", err)
		return
	}
	p.account = session
	p.observeAnalyticsOwner()
	p.refreshBackendReport()
}

// observeAnalyticsOwner reports only the account's one-way identifier; email and profile data stay local.
func (p *App) observeAnalyticsOwner() {
	if p.account != nil && p.analytics != nil {
		p.analytics.ObserveInstallOwner(p.account.Snapshot().UserID)
	}
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
		Packaged:      packagedBuild(),
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

// secondInstanceLink returns the nvm:// link on the command line of a second launch. The OS starts a new instance with
// the link there on Windows and Linux, where the running app must take it.
func secondInstanceLink(args []string) (string, bool) {
	prefix := auth.Scheme + "://"
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return arg, true
		}
	}
	return "", false
}

// receiveSecondInstanceLink hands the link of a second launch to the account session, off the UI thread.
func (p *App) receiveSecondInstanceLink(args []string) {
	link, ok := secondInstanceLink(args)
	if !ok || p.account == nil {
		return
	}
	account := p.account
	safego.Go("account.second_instance_link", func(context.Context) { account.HandleURL(link) })
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
	p.observeAnalyticsOwner()
	p.onUI(func() {
		p.syncCompatibilityAccount()
		p.Refresh()
		if p.chats != nil {
			p.chats.AccountChanged()
		}
	})
}

// onUI runs fn on the UI thread. Tests route it through uiQueue, which they drain; without a window, which only the
// goldens and tests have, fn runs directly.
func (p *App) onUI(fn func()) {
	switch {
	case p.uiQueue != nil:
		p.uiQueue.Update(fn)
	case p.panel != nil:
		p.panel.Update(fn)
	default:
		fn()
	}
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
