package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// setQueryRecorder is the palette host as a set-query action sees it: it records the query it was given.
type setQueryRecorder struct {
	fakeHost
	query *string
}

func (h *setQueryRecorder) SetQuery(q string) { *h.query = q }

func TestSetQueryActionSearchesAgainAndKeepsThePaletteOpen(t *testing.T) {
	var query string
	action := ActionFor(Deps{}, "nevermind.calculator", api.SetQuery("Continue", "4*2"))
	host := &setQueryRecorder{query: &query}
	action.Run(host)
	if query != "4*2" || host.hidden {
		t.Fatalf("query %q, hidden %v; want the query set and the palette open", query, host.hidden)
	}
}

func TestOpenPathActionHidesThePaletteAndOpensThePath(t *testing.T) {
	var opened []string
	deps := Deps{OpenPath: func(path string) { opened = append(opened, path) }}
	host := &hideRecorder{}
	ActionFor(deps, "acme.places", api.OpenPath("Open", "/Users/me/Downloads")).Run(host)
	if !host.hidden || len(opened) != 1 || opened[0] != "/Users/me/Downloads" {
		t.Fatalf("hidden %v, opened %v", host.hidden, opened)
	}
}

func TestRevealPathActionShowsThePathInTheFileManager(t *testing.T) {
	var revealed []string
	deps := Deps{RevealPath: func(path string) { revealed = append(revealed, path) }}
	host := &hideRecorder{}
	ActionFor(deps, "acme.files", api.RevealPath("Show in Finder", "/Users/me/a.txt")).Run(host)
	if !host.hidden || len(revealed) != 1 || revealed[0] != "/Users/me/a.txt" {
		t.Fatalf("hidden %v, revealed %v", host.hidden, revealed)
	}
}

func TestPathActionsWithoutAHostOperationAreUnavailable(t *testing.T) {
	var toasts []string
	host := &toastRecorder{toasts: &toasts}
	ActionFor(Deps{}, "acme.places", api.OpenPath("Open", "/x")).Run(host)
	if len(toasts) != 1 || toasts[0] != unavailableMessage {
		t.Fatalf("toasts = %v, want the unavailable message", toasts)
	}
}

func TestSystemOpRunsOnTheHostAndToastsItsFailure(t *testing.T) {
	var ran []string
	deps := Deps{RunSystem: func(op string, done func(string)) {
		ran = append(ran, op)
		done("Could not run sleep: boom")
	}}
	var toasts []string
	host := &systemHost{hideRecorder: hideRecorder{}, toasts: &toasts}
	ActionFor(deps, "nevermind.system-commands", api.SystemOp("Sleep", "sleep")).Run(host)
	if !host.hidden || len(ran) != 1 || ran[0] != "sleep" {
		t.Fatalf("hidden %v, ran %v", host.hidden, ran)
	}
	if len(toasts) != 1 || toasts[0] != "Could not run sleep: boom" {
		t.Fatalf("toasts = %v", toasts)
	}
}

func TestSystemOpSuccessShowsNothing(t *testing.T) {
	deps := Deps{RunSystem: func(_ string, done func(string)) { done("") }}
	var toasts []string
	ActionFor(deps, "nevermind.system-commands", api.SystemOp("Sleep", "sleep")).Run(&systemHost{toasts: &toasts})
	if len(toasts) != 0 {
		t.Fatalf("toasts = %v, want none", toasts)
	}
}

func TestDestructiveSystemOpAsksBeforeItRuns(t *testing.T) {
	var ran []string
	deps := Deps{RunSystem: func(op string, done func(string)) { ran = append(ran, op); done("") }}
	action := api.SystemOp("Empty Trash", "empty-trash")
	action.Confirm = &api.Confirmation{Title: "Empty Trash?", ConfirmLabel: "Empty Trash", Message: "This cannot be undone.", Destructive: true}
	host := &fakeHost{}
	ActionFor(deps, "nevermind.system-commands", action).Run(host)
	if len(ran) != 0 || len(host.pushed) != 1 || host.pushed[0].Placeholder != "Empty Trash?" {
		t.Fatalf("ran %v, pushed %d views; want the confirmation first", ran, len(host.pushed))
	}
	confirm := host.pushed[0].Search("")[0]
	if confirm.Title != "Empty Trash" || confirm.Subtitle != "This cannot be undone." || confirm.Actions[0].Title != "Confirm" {
		t.Fatalf("confirm row = %+v", confirm)
	}
}

// systemHost is the palette host for a system op: it hides, and records the toasts a failed op shows.
type systemHost struct {
	hideRecorder
	toasts *[]string
}

func (h *systemHost) Toast(msg string) { *h.toasts = append(*h.toasts, msg) }

var _ search.Host = (*systemHost)(nil)
