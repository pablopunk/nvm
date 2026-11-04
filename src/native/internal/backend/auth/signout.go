package auth

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// accountUnavailableMessage is what the account view says when the backend would not describe the account.
const accountUnavailableMessage = "Account details could not be read. Check your connection."

// SignOut ends the connection to the active backend. It revokes the token first, because the revocation needs the
// bearer it carries, then removes the token and the connection. The revocation is a network call, so callers run
// SignOut off the UI thread. It reports whether the backend confirmed the revocation; the local sign-out happens
// either way, as the desktop app does.
func (s *Session) SignOut(ctx context.Context) (revoked bool) {
	s.mu.Lock()
	client, origin := s.client, s.env.BaseURL
	s.mu.Unlock()

	revokeErr := client.RevokeCurrentToken(ctx)
	revoked = revokeErr == nil
	if !revoked {
		s.deps.Logger.Warn("auth.revoke_failed", logx.F{"kind": errorKind(revokeErr)})
	}
	if err := s.vault.DeleteToken(origin); err != nil {
		s.deps.Logger.Warn("auth.token_delete_failed", logx.F{"error": err.Error()})
	}

	s.mu.Lock()
	s.persistLocked(withoutConnection(origin))
	if s.client == client {
		s.connected = false
		s.connection = Connection{}
		s.account = AccountInfo{}
		s.notice = nil
	}
	s.mu.Unlock()
	s.notifyChange()
	return revoked
}

// Refresh reads the credit balance and the spend-cap state of the active backend, and the plan when the backend
// reports it. It makes network calls, so callers run it off the UI thread. A refusal from the backend is shown
// in the account view and returned.
func (s *Session) Refresh(ctx context.Context) error {
	client := s.Client()
	descriptor, err := client.ActiveModel(ctx, backend.ActiveModelQuery{})
	info := AccountInfo{}
	var notice *Notice
	if err != nil {
		info.Error = accountUnavailableMessage
		if spendCap, ok := SpendCapNoticeFromError(err); ok {
			notice = &spendCap
		}
	} else {
		info.Balance, info.HasBalance = balanceOf(descriptor)
		if spendCap, ok := SpendCapNoticeFromModel(descriptor); ok {
			notice = &spendCap
		}
	}
	if account, meErr := client.Me(ctx); meErr == nil {
		info.Plan = account.Plan
	}

	s.mu.Lock()
	if s.client == client {
		s.account = info
		s.notice = notice
	}
	s.mu.Unlock()
	s.notifyChange()
	return err
}

// NoteError records a spend-cap refusal from any request, so the account view can explain it. Other errors are
// ignored here; their callers show them where they happened.
func (s *Session) NoteError(err error) {
	notice, ok := SpendCapNoticeFromError(err)
	if !ok {
		return
	}
	s.mu.Lock()
	s.notice = &notice
	s.mu.Unlock()
	s.notifyChange()
}

// balanceOf returns the total credit balance a descriptor reports, and whether it reports one.
func balanceOf(descriptor backend.ModelDescriptor) (int, bool) {
	if descriptor.Credits == nil {
		return 0, false
	}
	return descriptor.Credits.Total, true
}

// withoutConnection returns a profile change that forgets the account of origin.
func withoutConnection(origin string) func(*Profile) {
	return func(p *Profile) {
		delete(p.Connections, origin)
	}
}
