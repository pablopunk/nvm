package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts/compositor"
)

// hyprlandCommandID names the Hyprland page's root command for shortcuts and aliases.
const hyprlandCommandID = "command:hyprland"

// hyprlandCommands offers the Hyprland page only in a Hyprland session; elsewhere the capability is omitted.
func (p *App) hyprlandCommands() []command {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil
	}
	return []command{completedNavigation(command{
		ID: hyprlandCommandID, Title: "Hyprland Integration", Icon: "monitor",
		Keywords: "hyprland compositor shortcuts keybinds window rules borders decorations config",
		Subtitle: "Shortcuts and window rules in your Hyprland config",
	}, func(h search.Host) { h.Push(p.hyprlandPage()) })}
}

// hyprlandPage shows what detection found and the setup, apply, undo and remove rows. It shows the cached state at
// once and probes again in the background.
func (p *App) hyprlandPage() *palette.View {
	page := &hyprlandPageState{p: p}
	view := &palette.View{Title: "Hyprland Integration", Placeholder: "Search Hyprland Integration"}
	view.Search = func(q string) []search.Item { return filterRows(page.rows(), q) }
	page.refresh()
	return view
}

// hyprlandPageState holds the page's latest preview, prepared off the UI thread.
type hyprlandPageState struct {
	p       *App
	desired atomic.Pointer[hyprland.Desired]
	plan    atomic.Pointer[hyprland.Plan]
	planErr atomic.Pointer[string]
	busy    atomic.Bool
}

// refresh detects again and prepares the preview, then redraws.
func (s *hyprlandPageState) refresh() {
	svc := s.p.compositorService()
	desired := s.p.compositorDesired()
	s.desired.Store(&desired)
	safego.Go("hyprland.detect", func(ctx context.Context) {
		svc.Detect(ctx)
		plan, err := svc.Preview(ctx, desired)
		if err != nil {
			msg := err.Error()
			s.planErr.Store(&msg)
			s.plan.Store(nil)
		} else {
			s.planErr.Store(nil)
			s.plan.Store(&plan)
		}
		s.p.onUI(s.p.Refresh)
	})
}

func (s *hyprlandPageState) rows() []search.Item {
	svc := s.p.compositorService()
	session, detected := svc.Session()
	rows := []search.Item{s.statusRow(session, detected)}
	if !detected {
		return rows
	}
	if plan := s.plan.Load(); plan != nil && session.Ready() {
		rows = append(rows, s.applyRow(*plan))
	} else if msg := s.planErr.Load(); msg != nil && session.Ready() {
		rows = append(rows, search.Item{Title: *msg, Contextual: true, Icon: "circle-alert"})
	}
	if session.Manual || (session.Ready() && s.plan.Load() != nil && !s.plan.Load().IncludePresent) {
		rows = append(rows, s.copyRows(session)...)
	}
	rows = append(rows, s.appearanceRow(svc))
	if session.Ready() {
		rows = append(rows,
			s.stepRow("Undo Last Change", "rotate-ccw", "Restore the module as it was before the last apply", svc.Undo),
			s.stepRow("Remove from Hyprland Config", "trash", "Delete the module and the include Nevermind added", svc.Remove))
	}
	return append(rows, reportRows(svc.Last())...)
}

func (s *hyprlandPageState) statusRow(session hyprland.Session, detected bool) search.Item {
	switch {
	case !detected:
		return search.Item{Title: "Checking Hyprland…", Icon: "loader", Contextual: true}
	case session.Problem != "" && session.Version == (hyprland.Version{}):
		return search.Item{Title: session.Problem, Icon: "circle-alert", Contextual: true}
	case session.Problem != "":
		return search.Item{Title: fmt.Sprintf("Hyprland %s: %s", session.Version, session.Problem), Icon: "circle-alert", Contextual: true}
	}
	return search.Item{
		Title: fmt.Sprintf("Hyprland %s, %s config", session.Version, session.Format), Subtitle: s.p.displayPath(session.Entry),
		Icon: "monitor", Contextual: true,
	}
}

// applyRow writes the module. Enter also adds the include when it is missing, which the preview names; the second
// action leaves the entry file alone.
func (s *hyprlandPageState) applyRow(plan hyprland.Plan) search.Item {
	preview := plan.Preview(true)
	row := search.Item{
		ID: "hyprland:apply", Title: "Apply Shortcuts and Window Rules", Icon: "check",
		Subtitle: s.applySubtitle(plan), Detail: textDetail(preview),
	}
	if plan.ModuleUnchanged() && plan.IncludePresent {
		row.Accessory = "Up to date"
	}
	row.Actions = []search.Action{{Title: "Apply", Dismiss: search.DismissNever, Run: func(h search.Host) { s.apply(h, plan, true) }}}
	if !plan.IncludePresent {
		row.Actions[0].Title = "Apply and Add Include"
		row.Actions = append(row.Actions, search.Action{Title: "Apply Without Changing " + s.p.displayPath(plan.Session.Entry), Dismiss: search.DismissNever, Run: func(h search.Host) { s.apply(h, plan, false) }})
	}
	return row
}

func (s *hyprlandPageState) applySubtitle(plan hyprland.Plan) string {
	module := s.p.displayPath(plan.Session.ModulePath)
	switch {
	case plan.ModuleUnchanged() && plan.IncludePresent:
		return module + " matches your shortcuts"
	case plan.IncludePresent:
		return "Update " + module
	}
	return "Write " + module + " and include it from " + s.p.displayPath(plan.Session.Entry)
}

func (s *hyprlandPageState) apply(h search.Host, plan hyprland.Plan, addInclude bool) {
	s.run(h, "Applying…", func(ctx context.Context) (hyprland.Result, error) {
		return s.p.compositorService().Apply(ctx, plan, addInclude)
	})
}

// run performs one lifecycle step off the UI thread, then toasts its outcome and redraws with a fresh preview. Steps
// do not overlap.
func (s *hyprlandPageState) run(h search.Host, busy string, step func(context.Context) (hyprland.Result, error)) {
	if !s.busy.CompareAndSwap(false, true) {
		h.Toast("Another Hyprland change is still running")
		return
	}
	h.Toast(busy)
	safego.Go("hyprland.step", func(ctx context.Context) {
		defer s.busy.Store(false)
		result, err := step(ctx)
		s.p.onUI(func() {
			h.Toast(stepMessage(result, err))
			s.p.resumeShortcutBindings()
			s.refresh()
		})
	})
}

// resumeShortcutBindings binds the shortcuts again, so ownership moves between the portal and the compositor.
func (p *App) resumeShortcutBindings() {
	if p.shortcutMgr != nil && !p.shortcutMgr.Suspended() {
		p.shortcutMgr.Suspend()
		p.summonChanged(p.shortcutMgr.Resume())
	}
}

func stepMessage(result hyprland.Result, err error) string {
	switch {
	case err != nil:
		return err.Error()
	case !result.Changed && result.Loaded == hyprland.LoadedConfirmed:
		return "Already up to date; Hyprland has the shortcuts"
	case !result.Changed:
		return "Nothing to change"
	case result.Loaded == hyprland.LoadedMissing:
		return "Written, but Hyprland does not list the shortcuts; check the include"
	case len(result.Conflicts) > 0:
		return fmt.Sprintf("Applied, with %d conflicting binding(s)", len(result.Conflicts))
	case result.Reloaded:
		return "Applied and reloaded Hyprland"
	}
	return "Written; Hyprland loads it once the include is in place"
}

func (s *hyprlandPageState) stepRow(title, icon, subtitle string, step func(context.Context) (hyprland.Result, error)) search.Item {
	return search.Item{Title: title, Icon: icon, Subtitle: subtitle, Actions: []search.Action{{
		Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.run(h, title+"…", step) },
	}}}
}

// appearanceRow turns the undecorated window rule on or off; the next apply writes it.
func (s *hyprlandPageState) appearanceRow(svc *compositor.Service) search.Item {
	on := svc.State().Undecorated
	accessory, next := "Off", "Turn On"
	if on {
		accessory, next = "On", "Turn Off"
	}
	return search.Item{
		Title: "Remove Window Borders", Icon: "square-dashed", Accessory: accessory,
		Subtitle: "Nevermind's windows without Hyprland's border, rounding, shadow and blur (at the next apply)",
		Actions: []search.Action{{Title: next, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := svc.SetUndecorated(!on); err != nil {
				h.Toast(err.Error())
				return
			}
			s.refresh()
			h.Refresh()
		}}},
	}
}

// copyRows give the manual setup: the include lines when the entry is not loaded automatically, and the whole module
// when Nevermind cannot write it.
func (s *hyprlandPageState) copyRows(session hyprland.Session) []search.Item {
	format := session.Format
	if format == "" {
		format = hyprland.FormatLua
	}
	module := session.ModulePath
	if module == "" {
		module = "nevermind" + format.Extension()
	}
	include := strings.Join(hyprland.IncludeLines(format, module), "\n")
	rows := []search.Item{{Title: "Copy Include Lines", Icon: "copy", Subtitle: "Add them to the end of your Hyprland config yourself", Detail: textDetail(include),
		Actions: []search.Action{{Title: "Copy", Run: func(h search.Host) { s.p.copyText(h, include) }}}}}
	if desired := s.desired.Load(); session.Manual && desired != nil {
		generated, _ := hyprland.Render(format, session.Version, *desired)
		rows = append(rows, search.Item{Title: "Copy Generated Config", Icon: "copy", Subtitle: "Save it as " + s.p.displayPath(module), Detail: textDetail(string(generated)),
			Actions: []search.Action{{Title: "Copy", Run: func(h search.Host) { s.p.copyText(h, string(generated)) }}}})
	}
	return rows
}

// reportRows show what the last step found: its error, conflicting binds, and what the module could not hold.
func reportRows(last *compositor.Report) []search.Item {
	if last == nil {
		return nil
	}
	var rows []search.Item
	if last.Err != nil {
		rows = append(rows, search.Item{Title: "Last " + last.Step + " failed: " + last.Err.Error(), Icon: "circle-alert", Contextual: true})
	}
	for _, c := range last.Result.Conflicts {
		rows = append(rows, search.Item{Title: c, Icon: "triangle-alert", Contextual: true})
	}
	for _, skipped := range last.Result.Skipped {
		rows = append(rows, search.Item{Title: "Not written: " + skipped.What + ": " + skipped.Reason, Icon: "triangle-alert", Contextual: true})
	}
	if last.Err == nil && last.Result.Loaded == hyprland.LoadedUnverified && last.Step == "apply" {
		rows = append(rows, search.Item{Title: "Hyprland's bind list could not confirm the shortcuts", Icon: "info", Contextual: true})
	}
	return rows
}

func textDetail(text string) func(c *ui.Context, _ search.Host) {
	return func(c *ui.Context, _ search.Host) {
		t := design.Tokens(c)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Text(c, text).FontSize(11).TextColor(t.Primary)
		})
	}
}

// filterRows keeps the rows whose title or subtitle contains the query.
func filterRows(rows []search.Item, q string) []search.Item {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	var out []search.Item
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.Title+" "+row.Subtitle), q) {
			out = append(out, row)
		}
	}
	return out
}
