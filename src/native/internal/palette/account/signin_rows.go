package account

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/search"
)

// connectedRows shows the signed-in account, its details, and the way out.
func connectedRows(s auth.Snapshot, a Actions) []search.Item {
	items := []search.Item{{
		Title:    "Logged in as " + s.Email,
		Subtitle: accountSummary(s),
		Icon:     "user",
	}}
	if s.Account.Error != "" {
		items = append(items, search.Item{Title: "Account details unavailable", Subtitle: s.Account.Error, Icon: "bell"})
	}
	items = append(items,
		search.Item{Title: "Refresh account details", Subtitle: "Credits and plan from the backend", Icon: "clock", Actions: actionIf(a.Refresh)},
		search.Item{Title: "Log out of Nevermind", Subtitle: "Signed in as " + s.Email, Icon: "user", Actions: actionIf(a.SignOut)},
	)
	return items
}

// accountSummary is the one-line account description: the plan, which reads unknown until a route reports it, and
// the balance when the backend reported it.
func accountSummary(s auth.Snapshot) string {
	summary := "Signed in as " + s.Email + " · " + planLabel(s.Account.Plan)
	if s.Account.HasBalance {
		summary += fmt.Sprintf(" · %d credits", s.Account.Balance)
	}
	return summary
}

// planLabel names the plan the account view shows.
func planLabel(plan string) string {
	if plan == "" {
		return "Plan unknown"
	}
	return "Plan " + plan
}

// signedOutRows shows the sign-in as it stands: the way in, a flow in progress, or how the last flow ended.
func signedOutRows(s auth.Snapshot, a Actions) []search.Item {
	switch s.SignIn.Phase {
	case auth.PhaseStarting:
		return []search.Item{{Title: "Preparing sign-in", Subtitle: "Requesting a one-time verification code…", Icon: "user"}}
	case auth.PhasePending:
		return pendingRows(s.SignIn, a)
	case auth.PhaseDenied:
		return endedRows(a, "Sign-in was denied", "The approval was refused. Start a new sign-in to try again.")
	case auth.PhaseExpired:
		return endedRows(a, "Sign-in code expired", "This one-time code expired. Start a new sign-in to try again.")
	case auth.PhaseCancelled:
		return endedRows(a, "Sign-in cancelled", "The sign-in request was cancelled. Start a new sign-in to try again.")
	case auth.PhaseFailed:
		return endedRows(a, "Sign-in could not be started", s.SignIn.Message)
	default:
		return []search.Item{{
			Title:    "Log in to Nevermind",
			Subtitle: "Connect this device to your Nevermind account",
			Icon:     "user",
			Actions:  actionIf(a.StartSignIn),
		}}
	}
}

// pendingRows shows the code and the page while the user approves in the browser.
func pendingRows(sign auth.SignInState, a Actions) []search.Item {
	title, subtitle := "Waiting for approval", "Approve this device in your browser. This view updates automatically."
	if sign.BrowserOpenFailed {
		title, subtitle = "Browser could not be opened", "Open the verification URL manually, or retry opening it below."
	}
	return []search.Item{
		{Title: title, Subtitle: subtitle, Icon: "user"},
		{Title: "Verification URL", Subtitle: sign.VerificationURL, Icon: "link"},
		{Title: "One-time user code", Subtitle: sign.Code, Icon: "lock"},
		{Title: "Open verification page", Subtitle: "Retry opening the sign-in page in your browser", Icon: "link", Actions: actionIf(a.OpenVerification)},
		{Title: "Cancel sign-in", Subtitle: "Stop waiting for device approval", Icon: "x", Actions: actionIf(a.CancelSignIn)},
	}
}

// endedRows explains a sign-in that will not complete, and offers a new one.
func endedRows(a Actions, title, subtitle string) []search.Item {
	return []search.Item{
		{Title: title, Subtitle: subtitle, Icon: "user"},
		{Title: "Start a new sign-in", Subtitle: "Request a new one-time code", Icon: "user", Actions: actionIf(a.StartSignIn)},
	}
}
