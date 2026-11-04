package app

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// accountCommands are the root commands that log in or out, as 1.x's account root item does. Only the one the session
// needs is listed: Log in while signed out, Log out while signed in.
func (p *App) accountCommands() []command {
	if p.signedIn() {
		return []command{{Title: "Log out of Nevermind", Keywords: "logout log out sign out account nevermind disconnect", Icon: "user",
			Subtitle: "Signed in as " + p.accountSource().Email, Run: p.signOutFromRoot}}
	}
	return []command{{Title: "Log in to Nevermind", Keywords: "login log in sign in account nevermind connect", Icon: "user",
		Subtitle: "Connect this device to your Nevermind account", Dismiss: search.DismissNever, Run: p.signInFromRoot}}
}

// signInFromRoot opens the account page on its sign-in flow, which shows the verification page and code.
func (p *App) signInFromRoot(h search.Host) {
	h.Push(p.accountView())
	p.accountActions().StartSignIn(h)
}

// signOutFromRoot signs the session out off the UI thread, then reports the result in the HUD, which outlives the
// palette's hide.
func (p *App) signOutFromRoot(search.Host) {
	session := p.account
	if session == nil {
		return
	}
	email := p.accountSource().Email
	safego.Go("account.signout", func(ctx context.Context) {
		revoked := session.SignOut(ctx)
		p.onUI(func() {
			p.refreshAccountPage()
			p.showFeedback("Nevermind Account", signOutMessage(email, revoked))
		})
	})
}

// signOutMessage says what signing out did. A token the backend did not revoke is said out loud, because the session
// on the server may still be live.
func signOutMessage(email string, revoked bool) string {
	if revoked {
		return fmt.Sprintf("Logged out of %s", email)
	}
	return fmt.Sprintf("Logged out of %s (token revoke failed - check connection)", email)
}
