package aichat

import (
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/design"
)

// Copy of the sign-in gate a chat shows while no account is connected. It is the gate the Electron app shows, and Enter
// on the page starts the sign-in.
const (
	signInTitle     = "Sign in to Nevermind"
	signInSubtitle  = "Connect this device to your Nevermind account to use AI chats."
	toastSignInBusy = "A sign-in is already running. Approve it in your browser."
)

// signInStatus says where the sign-in stands, under the gate's subtitle. An idle flow has nothing to say.
func signInStatus(s auth.SignInState) string {
	switch s.Phase {
	case auth.PhaseStarting:
		return "Requesting a one-time sign-in code…"
	case auth.PhasePending:
		return "Approve this device in your browser. This view updates automatically."
	case auth.PhaseDenied:
		return "Sign-in was denied. Press Enter to try again."
	case auth.PhaseExpired:
		return "This one-time code expired. Press Enter to start a new sign-in."
	case auth.PhaseCancelled:
		return "The sign-in was cancelled. Press Enter to try again."
	case auth.PhaseFailed:
		return s.Message
	}
	return ""
}

// signInRefusal is the palette line for a sign-in that did not start.
func signInRefusal(err error) string {
	if errors.Is(err, auth.ErrSignInInProgress) {
		return toastSignInBusy
	}
	return err.Error()
}

// signInGate draws the gate in place of the transcript: the title, the subtitle, and the flow's status.
func signInGate(c *ui.Context, state auth.SignInState, k *design.TokenSet) {
	status := signInStatus(state)
	ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
		ui.Text(c, signInTitle).FontSize(13).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, signInSubtitle).FontSize(13).TextColor(k.Tertiary)
		if status != "" {
			ui.Text(c, status).FontSize(12).TextColor(k.Secondary)
		}
	})
}
