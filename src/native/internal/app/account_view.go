package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/palette"
	palaccount "github.com/pablopunk/nvm/native/internal/palette/account"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// accountSource reads the session for the account page. Before the session exists the page reads as signed out.
func (p *App) accountSource() auth.Snapshot {
	if p.account == nil {
		return auth.Snapshot{}
	}
	return p.account.Snapshot()
}

// accountView is the account page over the live session.
func (p *App) accountView() *palette.View {
	return palaccount.View(p.accountSource, p.accountActions())
}

// openAccountPage pushes the account page and refreshes the balance in the background, so the page shows the
// last known numbers at once.
func (p *App) openAccountPage(h search.Host) {
	h.Push(p.accountView())
	if p.signedIn() {
		p.refreshAccount(h)
	}
}

// refreshAccount reads the balance and notices from the backend, then redraws the page the host shows.
func (p *App) refreshAccount(h search.Host) {
	session := p.account
	safego.Go("account.refresh", func(ctx context.Context) {
		_ = session.Refresh(ctx)
		p.onUI(h.Refresh)
	})
}

// accountActions maps the page's rows onto the session. Network work runs off the UI thread; the page redraws
// when it returns.
func (p *App) accountActions() palaccount.Actions {
	if p.account == nil {
		return palaccount.Actions{}
	}
	session := p.account
	return palaccount.Actions{
		StartSignIn: func(h search.Host) {
			p.startSignIn(h, session.StartSignIn)
		},
		CancelSignIn: func(h search.Host) {
			session.CancelSignIn()
			h.Refresh()
		},
		OpenVerification: func(h search.Host) {
			reportIfFailed(h, session.RetryOpenVerificationPage())
			h.Refresh()
		},
		SignOut: func(h search.Host) {
			safego.Go("account.signout", func(ctx context.Context) {
				session.SignOut(ctx)
				p.onUI(h.Refresh)
			})
		},
		Refresh: func(h search.Host) {
			p.refreshAccount(h)
		},
		SetBYOKey: func(h search.Host) {
			h.Push(p.byoKeyForm())
		},
		ClearBYOKey: func(h search.Host) {
			reportIfFailed(h, session.ClearBYOKey())
			h.Refresh()
		},
		SwitchEnvironment: func(h search.Host, kind backend.EnvironmentKind, rawURL string) {
			reportIfFailed(h, session.SwitchEnvironment(kind, rawURL))
			h.Refresh()
		},
		Development: !packagedBuild(),
	}
}

// byoKeyForm asks for the provider, model, origin and key. The key is the only secret; the form saves it to the
// keychain and nowhere else.
func (p *App) byoKeyForm() *palette.View {
	fields := []*FormField{
		{Label: "Provider", Placeholder: "openai"},
		{Label: "Model", Placeholder: "model id"},
		{Label: "Base URL", Placeholder: "https://api.example.com/v1"},
		{Label: "API key", Placeholder: "key"},
	}
	return p.formView("Own Provider Key", fields, func(vals []string) string {
		if p.account == nil {
			return "The account is not available"
		}
		err := p.account.SetBYOKey(auth.BYOInput{Provider: vals[0], ModelID: vals[1], BaseURL: vals[2], APIKey: vals[3]})
		if err != nil {
			return err.Error()
		}
		return ""
	})
}

// reportIfFailed shows err as a toast on the page, when there is one.
func reportIfFailed(h search.Host, err error) {
	if err != nil {
		h.Toast(err.Error())
	}
}

// startSignIn checks that the backend accepts this client before the sign-in starts, as 1.x does. The check fetches the
// manifest, so it runs off the UI thread, and the sign-in starts or the refusal shows when it answers.
func (p *App) startSignIn(h search.Host, start func() error) {
	go func() {
		err := p.checkCompatibility(context.Background())
		p.onUI(func() {
			if err == nil {
				err = start()
			}
			reportIfFailed(h, err)
			h.Refresh()
		})
	}()
}
