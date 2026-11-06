package app

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"

	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// accountCommands are the root commands that log in or out, as 1.x's account root item does. Only the one the session
// needs is listed: Log in while signed out, Log out while signed in.
func (p *App) accountCommands() []command {
	return append(p.signInOrOutCommands(), p.backendRootCommands()...)
}

// signInOrOutCommands is the one sign-in row the session needs.
func (p *App) signInOrOutCommands() []command {
	if p.signedIn() {
		return []command{{ID: shortcuts.CommandSignInOut, Title: "Log out of Nevermind", Keywords: "logout log out sign out account nevermind disconnect", Icon: "user",
			Subtitle: "Signed in as " + p.accountSource().Email, Run: p.signOutFromRoot, Complete: p.signOutFromRootWithCompletion}}
	}
	return []command{{ID: shortcuts.CommandSignInOut, Title: "Log in to Nevermind", Keywords: "login log in sign in account nevermind connect", Icon: "user",
		Subtitle: "Connect this device to your Nevermind account", Dismiss: search.DismissNever, Run: p.signInFromRoot}}
}

// signInFromRoot opens the account page on its sign-in flow, which shows the verification page and code.
func (p *App) signInFromRoot(h search.Host) {
	h.Push(p.accountView())
	p.accountActions().StartSignIn(h)
}

// signOutFromRoot signs the session out off the UI thread, then reports the result in the HUD, which outlives the
// palette's hide.
func (p *App) signOutFromRoot(h search.Host) {
	p.signOutFromRootWithCompletion(context.Background(), h, func(analytics.OperationOutcome) {})
}

func (p *App) signOutFromRootWithCompletion(ctx context.Context, _ search.Host, complete func(analytics.OperationOutcome)) {
	session := p.account
	if session == nil {
		complete(analytics.OutcomeBlocked)
		return
	}
	email := p.accountSource().Email
	safego.Go("account.signout", func(ctx context.Context) {
		revoked := session.SignOut(ctx)
		outcome := analytics.OutcomeFailed
		if revoked {
			outcome = analytics.OutcomeSuccess
		} else if ctx.Err() == context.Canceled {
			outcome = analytics.OutcomeCancelled
		} else if ctx.Err() == context.DeadlineExceeded {
			outcome = analytics.OutcomeTimedOut
		}
		complete(outcome)
		p.onUI(func() {
			p.refreshAccountPage()
			p.showFeedback("Nevermind Account", signOutMessage(email, revoked), signOutTone(revoked))
		})
	}, safego.WithContext(ctx))
}

// signOutTone reads a sign-out the server did not revoke as an error, as 1.x does.
func signOutTone(revoked bool) feedback.Tone {
	if revoked {
		return feedback.ToneSuccess
	}
	return feedback.ToneError
}

// signOutMessage says what signing out did. A token the backend did not revoke is said out loud, because the session
// on the server may still be live.
func signOutMessage(email string, revoked bool) string {
	if revoked {
		return fmt.Sprintf("Logged out of %s", email)
	}
	return fmt.Sprintf("Logged out of %s (token revoke failed - check connection)", email)
}
