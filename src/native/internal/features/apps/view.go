package apps

import (
	"fmt"
	"strings"
)

// The apps views as plain data: rows, actions, review lists and results. The composition root maps them onto the
// palette, so this package never imports the palette, the search layer or the UI toolkit.

// ActionKind names what an app action does. The composition root decides how each kind runs.
type ActionKind string

// Action kinds of app rows and of the app views.
const (
	ActionOpen      ActionKind = "open"
	ActionReveal    ActionKind = "reveal"
	ActionCopyPath  ActionKind = "copy-path"
	ActionQuit      ActionKind = "quit"
	ActionForceQuit ActionKind = "force-quit"
	ActionUninstall ActionKind = "uninstall"
	ActionQuitAll   ActionKind = "quit-all"
	ActionOpenTrash ActionKind = "open-trash"
)

// Action is one thing a row can do. Destructive actions ask for confirmation through the palette before they run.
type Action struct {
	Kind        ActionKind
	Title       string
	Destructive bool
	// Confirm is the question asked before a destructive action runs.
	Confirm        string
	ConfirmMessage string
}

// Subtitles of app rows. "Running" is the only state the row shows; an idle app has no subtitle, as in 1.x.
const RunningSubtitle = "Running"

// Row is an app as the palette lists it. ID is stable across refreshes; Path is what Open and Reveal act on.
type Row struct {
	ID       string
	Title    string
	Subtitle string
	Path     string
	Running  bool
	Actions  []Action
}

// AppRows lists the apps in index order with their actions, as 1.x lists them: Open first, then Uninstall when the
// platform supports it. The rest of an app's menu is the shared customization set (shortcut and alias) the palette adds.
func AppRows(list []App, running map[string]bool, uninstallable bool) []Row {
	rows := make([]Row, 0, len(list))
	for _, app := range list {
		rows = append(rows, appRow(app, running[app.Path], uninstallable))
	}
	return rows
}

func appRow(app App, isRunning, uninstallable bool) Row {
	row := Row{ID: "app:" + app.Path, Title: app.Name, Path: app.Path, Running: isRunning}
	if isRunning {
		row.Subtitle = RunningSubtitle
	}
	row.Actions = []Action{{Kind: ActionOpen, Title: "Open " + app.Name}}
	if uninstallable {
		row.Actions = append(row.Actions, Action{Kind: ActionUninstall, Title: "Uninstall " + app.Name + "…", Destructive: true})
	}
	return row
}

// forceQuitAction is the destructive quit: it needs the user's confirmation, because unsaved work is lost.
func forceQuitAction(name string) Action {
	return Action{
		Kind:           ActionForceQuit,
		Title:          "Force Quit " + name,
		Destructive:    true,
		Confirm:        "Force Quit " + name,
		ConfirmMessage: "Force quit " + name + "? Unsaved data may be lost.",
	}
}

// ForceQuitRows lists the running apps of the index, for the Force Quit Apps view. Each row offers Quit and Force
// Quit; nothing else.
func ForceQuitRows(list []App, running map[string]bool) []Row {
	var rows []Row
	for _, app := range list {
		if !running[app.Path] {
			continue
		}
		rows = append(rows, Row{
			ID:       "force-quit:" + app.Path,
			Title:    app.Name,
			Subtitle: RunningSubtitle,
			Path:     app.Path,
			Running:  true,
			Actions: []Action{
				{Kind: ActionQuit, Title: "Quit " + app.Name},
				forceQuitAction(app.Name),
			},
		})
	}
	return rows
}

// QuitAllPlan splits the running apps into the ones "Quit All Apps" asks to close and the ones it leaves alone.
// Nevermind is always left alone; the exception list (QuitExceptions) names the desktop shell and file manager.
func QuitAllPlan(list []App, running map[string]bool, self App) (quit, excepted []App) {
	for _, app := range list {
		if !running[app.Path] || app.Path == self.Path {
			continue
		}
		if IsExcludedFromQuitAll(app.Name) {
			excepted = append(excepted, app)
			continue
		}
		quit = append(quit, app)
	}
	return quit, excepted
}

// NoSelectionMessage is shown when the review is confirmed with nothing selected. Nothing is moved in that case.
const NoSelectionMessage = "Select at least one item to move to Trash"

// ReviewItem is one candidate in the uninstall review: its path as shown, why it is listed, and whether it is set
// to move to the Trash.
type ReviewItem struct {
	ID       string
	Title    string
	Subtitle string
	Kind     string
	Selected bool
}

// Review is the uninstall review: the notes discovery made, and the items with their toggles.
type Review struct {
	Title       string
	Notes       []string
	Items       []ReviewItem
	Selected    int
	SelectedAll []Candidate
}

// BuildReview lists the candidates of a ready discovery with their current selection. displayPath shortens a path
// for people (the home folder as "~").
func BuildReview(discovery Discovery, selected map[string]bool, displayPath func(string) string) Review {
	review := Review{Title: "Uninstall " + appNameOf(discovery.Snapshot.AppPath)}
	for _, note := range discovery.Notes {
		review.Notes = append(review.Notes, note.Message)
	}
	for _, candidate := range discovery.Snapshot.Candidates {
		on := selected[candidate.ID]
		if on {
			review.Selected++
		}
		review.Items = append(review.Items, ReviewItem{
			ID:       candidate.ID,
			Title:    displayPath(candidate.Path),
			Subtitle: candidate.Reason,
			Kind:     candidate.Kind,
			Selected: on,
		})
	}
	return review
}

// ConfirmText is the question before the Trash run: the selected paths, and a line when the app itself will be
// quit. The wording follows 1.x.
func ConfirmText(selected []Candidate, displayPath func(string) string) (title, message string) {
	count := len(selected)
	noun := "items"
	if count == 1 {
		noun = "item"
	}
	title = fmt.Sprintf("Move %d %s to Trash", count, noun)
	lines := make([]string, 0, count+1)
	quitsApp := false
	for _, candidate := range selected {
		lines = append(lines, displayPath(candidate.Path))
		if candidate.Kind == KindApp {
			quitsApp = true
		}
	}
	if quitsApp {
		lines = append(lines, "If running, the application will be asked to quit first.")
	}
	return title, strings.Join(lines, "\n")
}

// Result is the outcome of a Trash run as the review shows it.
type Result struct {
	Title   string
	Message string
	// Success is true when every selected item moved; the palette then shows a short confirmation instead of a list.
	Success bool
	Summary string
	Details []ResultDetail
}

// ResultDetail is one line of the result list: an item that did not move, or one that did.
type ResultDetail struct {
	Title    string
	Subtitle string
}

// BuildResult turns a run's outcome into the result view. A complete run is a one-line summary; a partial or failed
// run lists what stayed in place (with its reason) and what moved.
func BuildResult(res TrashResult, displayPath func(string) string) Result {
	if res.Status == StatusComplete {
		noun := "items"
		if len(res.Moved) == 1 {
			noun = "item"
		}
		summary := fmt.Sprintf("%d %s moved to Trash", len(res.Moved), noun)
		return Result{Title: "Uninstall", Success: true, Summary: summary}
	}
	out := Result{Title: "Nothing was moved to Trash", Message: "Selected items were left untouched."}
	if res.Status == StatusPartial {
		out.Title = "Uninstall partially completed"
		out.Message = "Some selected items were left untouched."
	}
	for _, item := range res.Untouched {
		out.Details = append(out.Details, ResultDetail{Title: displayPath(item.Path), Subtitle: item.Message})
	}
	for _, moved := range res.Moved {
		out.Details = append(out.Details, ResultDetail{Title: displayPath(moved), Subtitle: "Moved to Trash"})
	}
	return out
}

// DisplayPath shortens a path under the home folder to "~".
func DisplayPath(p, home string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
