package app

import (
	"context"
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"

	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// The root scores of the two backend rows, as 1.x's account items have them.
const (
	backendEnvironmentRootScore = 20
	backendStatusRootScore      = 6
)

// environmentChoice is one way to point the app at a backend. A choice that names its origin asks for it in a form
// whose field is fieldLabel; the others switch at once.
type environmentChoice struct {
	title, subtitle string
	kind            backend.EnvironmentKind
	fieldLabel      string
	placeholder     string
}

// environmentChoices lists the backends the switch offers, in 1.x's order. Development is offered only in an unpackaged
// build.
func environmentChoices(packaged bool) []environmentChoice {
	var out []environmentChoice
	if !packaged {
		out = append(out, environmentChoice{title: "Development", subtitle: "http://localhost:4321", kind: backend.EnvironmentDevelopment})
	}
	return append(out,
		environmentChoice{title: "Preview URL…", subtitle: "Use a Vercel Preview deployment", kind: backend.EnvironmentPRPreview,
			fieldLabel: "Preview URL", placeholder: "https://nvm-your-branch.vercel.app"},
		environmentChoice{title: "Production", subtitle: "https://api.nvm.fyi", kind: backend.EnvironmentProduction},
		environmentChoice{title: "Custom URL…", subtitle: "Use a validated HTTPS backend URL", kind: backend.EnvironmentCustom,
			fieldLabel: "Backend URL", placeholder: "https://your-preview.vercel.app"},
	)
}

// backendRootCommands are the two backend rows of the root: the environment switch, and the status, which is always
// listed and says so when the server and the app disagree.
func (p *App) backendRootCommands() []command {
	return []command{
		{ID: shortcuts.CommandSwitchBackend, Title: "Nevermind: Switch Backend Environment", Keywords: "backend environment production preview custom url", Icon: "globe",
			Subtitle: "Switch Development, Preview, Production, or custom backend", rootScore: backendEnvironmentRootScore,
			Run: func(h search.Host) { h.Push(p.backendEnvironmentView()) }},
		p.backendStatusCommand(),
	}
}

// backendStatusCommand is the status row. It opens nothing; it reads the session and the last report the backend gave.
func (p *App) backendStatusCommand() command {
	snap := p.accountSource()
	title, subtitle, icon := backendStatusRow(snap.Environment, snap.BaseURL, p.lastBackendReport())
	return command{ID: shortcuts.CommandBackendStatus, Title: title, Keywords: "debug backend status environment status", Icon: icon, Subtitle: subtitle,
		rootScore: backendStatusRootScore, Dismiss: search.DismissNever, Run: func(search.Host) {}}
}

// backendStatusRow words the status row as 1.x does: the environment the app uses, its origin, and the server's
// environment and version, or unavailable without a report. A server on another environment marks the title.
func backendStatusRow(active backend.EnvironmentKind, baseURL string, report *backend.CompatReport) (title, subtitle, icon string) {
	server := "unavailable"
	mismatch := false
	if report != nil && report.Manifest.Backend.Environment != "" && report.Manifest.Backend.Version != "" {
		served := report.Manifest.Backend.Environment
		server = served + " (" + report.Manifest.Backend.Version + ")"
		mismatch = serverEnvironmentKind(served) != active
	}
	title, icon = "Nevermind: Backend Status", "globe"
	if mismatch {
		title, icon = title+" — mismatch", "circle-alert"
	}
	subtitle = "Client " + string(active) + " · Active " + string(active) + " · " + baseURL + " · Server " + server
	return title, subtitle, icon
}

// serverEnvironmentKind maps the environment name a server reports to the app's kind; a preview server is named preview.
func serverEnvironmentKind(served string) backend.EnvironmentKind {
	if served == "preview" {
		return backend.EnvironmentPRPreview
	}
	return backend.EnvironmentKind(served)
}

// backendEnvironmentView is the list of backends the environment switch offers, filtered by the query.
func (p *App) backendEnvironmentView() *palette.View {
	return &palette.View{Title: "Switch Backend Environment", Placeholder: "Choose an environment", Search: func(q string) []search.Item {
		var out []search.Item
		for _, choice := range environmentChoices(packagedBuild()) {
			if q != "" && search.Score(choice.title, strings.ToLower(q)) == 0 {
				continue
			}
			out = append(out, p.environmentItem(choice))
		}
		return out
	}}
}

// environmentItem is the row of one choice: it switches at once, or asks for the origin first.
func (p *App) environmentItem(choice environmentChoice) search.Item {
	run := func(h search.Host) { p.switchBackendFromHost(h, choice.kind, "") }
	if choice.fieldLabel != "" {
		run = func(h search.Host) { h.Push(p.backendOriginForm(choice)) }
	}
	return search.Item{Kind: search.KindCommands, Title: choice.title, Subtitle: choice.subtitle, Icon: "globe",
		Actions: []search.Action{{Title: "Use", Dismiss: search.DismissNever, Run: run}}}
}

// backendOriginForm asks for the origin of a preview or custom backend, and switches to it on save.
func (p *App) backendOriginForm(choice environmentChoice) *palette.View {
	fields := []*FormField{{Label: choice.fieldLabel, Placeholder: choice.placeholder}}
	return p.formView(choice.title, fields, func(vals []string) string {
		if err := p.switchBackend(choice.kind, vals[0]); err != nil {
			return err.Error()
		}
		p.reportBackendSwitch(nil)
		p.Hide()
		return ""
	})
}

// switchBackendFromHost switches to a backend from a row and reports the result in the HUD, then hides the palette.
func (p *App) switchBackendFromHost(h search.Host, kind backend.EnvironmentKind, rawURL string) {
	p.reportBackendSwitch(p.switchBackend(kind, rawURL))
	h.Hide()
}

// switchBackend makes the backend the active one and asks the backend for its report again.
func (p *App) switchBackend(kind backend.EnvironmentKind, rawURL string) error {
	if p.account == nil {
		//lint:ignore ST1005 user-facing copy, shown as written
		return errors.New("The account is not available")
	}
	if err := p.account.SwitchEnvironment(kind, rawURL); err != nil {
		return err
	}
	p.refreshBackendReport()
	return nil
}

// reportBackendSwitch shows the outcome of a switch in the HUD, as 1.x's Nevermind Backend feedback does.
func (p *App) reportBackendSwitch(err error) {
	if err != nil {
		p.showFeedback("Nevermind Backend", err.Error(), feedback.ToneError)
		return
	}
	p.showFeedback("Nevermind Backend", "Switched the backend", feedback.ToneSuccess)
}

// refreshBackendReport asks the active backend for its compatibility report in the background. The status row shows
// the last report it got; a failed request keeps the one it had.
func (p *App) refreshBackendReport() {
	if p.account == nil {
		return
	}
	client := p.account.Client()
	if client == nil {
		return
	}
	safego.Go("backend.compatibility", func(ctx context.Context) {
		report, err := client.Compatibility(ctx)
		if err == nil {
			p.backendReport.Store(&report)
		}
	})
}

// lastBackendReport is the report the status row shows, or nil before the backend has given one.
func (p *App) lastBackendReport() *backend.CompatReport {
	return p.backendReport.Load()
}
