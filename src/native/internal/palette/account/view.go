// Package account is the palette page for the Nevermind account: sign-in, sign-out, the backend, the bring-your-own
// key and the spend-cap notice. It draws the state of the auth session and never holds the session itself; the
// composition root supplies the source of state and the actions that change it.
package account

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Source returns the current state of the account. The palette searches on the UI thread, so it must not block.
type Source func() auth.Snapshot

// Actions are the operations the rows run. Each receives the palette host so it can refresh the page, toast a
// message or push a form. A nil action leaves its row without an action.
type Actions struct {
	StartSignIn      func(h search.Host)
	CancelSignIn     func(h search.Host)
	OpenVerification func(h search.Host)
	SignOut          func(h search.Host)
	Refresh          func(h search.Host)
	SetBYOKey        func(h search.Host)
	ClearBYOKey      func(h search.Host)
	// SwitchEnvironment changes the backend. Kind and origin come from the environment page.
	SwitchEnvironment func(h search.Host, kind backend.EnvironmentKind, rawURL string)
	// Development offers the loopback backend, which only unpackaged builds may use.
	Development bool
}

// View is the account page. Its rows follow the source each time the palette searches, so a page kept on the
// stack shows the current state.
func View(source Source, actions Actions) *palette.View {
	return &palette.View{
		Title:       "Nevermind Account",
		Placeholder: "Filter account",
		Search: func(q string) []search.Item {
			return filterRows(rows(source, actions), q)
		},
	}
}

// EnvironmentView lists the backends the app can talk to, and offers the typed text as a preview or custom origin.
func EnvironmentView(source Source, actions Actions) *palette.View {
	return &palette.View{
		Title:       "Switch Backend Environment",
		Placeholder: "Presets, or type a backend URL",
		Search: func(q string) []search.Item {
			return environmentRows(source(), actions, strings.TrimSpace(q))
		},
	}
}

// filterRows keeps the rows whose title contains q, ignoring case. An empty q keeps every row.
func filterRows(items []search.Item, q string) []search.Item {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	kept := items[:0]
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), q) {
			kept = append(kept, item)
		}
	}
	return kept
}

// rows is the account page: the account or the sign-in, then the backend, the BYO key and the notice.
func rows(source Source, a Actions) []search.Item {
	s := source()
	var items []search.Item
	if s.Connected {
		items = connectedRows(s, a)
	} else {
		items = signedOutRows(s, a)
	}
	items = append(items, backendRow(s, source, a), byoRow(s, a))
	if s.Notice != nil {
		items = append(items, search.Item{Title: s.Notice.Title, Subtitle: s.Notice.Message, Icon: "bell", Tone: "orange"})
	}
	return items
}

// environmentRows lists the presets, and the typed text as a preview or custom origin.
func environmentRows(s auth.Snapshot, a Actions, typed string) []search.Item {
	var items []search.Item
	if a.Development {
		items = append(items, environmentRow(s, a, "Development", "http://localhost:4321", backend.EnvironmentDevelopment, ""))
	}
	items = append(items, environmentRow(s, a, "Production", "https://api.nvm.fyi", backend.EnvironmentProduction, ""))
	if typed != "" {
		items = append(items,
			environmentRow(s, a, "Use preview URL “"+typed+"”", typed, backend.EnvironmentPRPreview, typed),
			environmentRow(s, a, "Use custom URL “"+typed+"”", typed, backend.EnvironmentCustom, typed),
		)
	}
	return items
}

// environmentRow is one backend choice. Its action switches to it and leaves the page open.
func environmentRow(s auth.Snapshot, a Actions, title, origin string, kind backend.EnvironmentKind, rawURL string) search.Item {
	return search.Item{
		Title:     title,
		Subtitle:  origin,
		Icon:      "server",
		Accessory: current(s.Environment == kind && s.BaseURL == origin),
		Actions:   switchActions(a, kind, rawURL),
	}
}

// switchActions is the action of an environment row, or none when switching is not wired.
func switchActions(a Actions, kind backend.EnvironmentKind, rawURL string) []search.Action {
	if a.SwitchEnvironment == nil {
		return nil
	}
	return []search.Action{{Title: "Switch", Dismiss: search.DismissNever, Run: func(h search.Host) {
		a.SwitchEnvironment(h, kind, rawURL)
	}}}
}

// backendRow opens the environment page, and shows the origin the app talks to.
func backendRow(s auth.Snapshot, source Source, a Actions) search.Item {
	return search.Item{
		Title:     "Backend",
		Subtitle:  s.BaseURL,
		Icon:      "server",
		Accessory: string(s.Environment),
		Actions: []search.Action{{Title: "Switch", Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(EnvironmentView(source, a))
		}}},
	}
}

// byoRow sets the user's own provider key, or clears the one in use.
func byoRow(s auth.Snapshot, a Actions) search.Item {
	if s.BYO == nil {
		return search.Item{
			Title:    "Use your own provider key",
			Subtitle: "Kept in the keychain; requests go straight to your provider",
			Icon:     "key",
			Actions:  actionIf(a.SetBYOKey),
		}
	}
	return search.Item{
		Title:    "Clear BYO provider key",
		Subtitle: "Using own key for " + s.BYO.Provider,
		Icon:     "key",
		Actions:  actionIf(a.ClearBYOKey),
	}
}

// actionIf is the one action a row runs, or none when the action is not wired.
func actionIf(run func(h search.Host)) []search.Action {
	if run == nil {
		return nil
	}
	return []search.Action{{Title: "Run", Dismiss: search.DismissNever, Run: run}}
}

// current marks the backend in use.
func current(yes bool) string {
	if yes {
		return "Current"
	}
	return ""
}
