package extview

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestUpdateActionRunsItsOperationOnTheHost(t *testing.T) {
	var ran []string
	deps := Deps{RunUpdate: func(op, _ string) error { ran = append(ran, op); return nil }}
	host := &refreshRecorder{toastRecorder: toastRecorder{toasts: new([]string)}}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpCheck, Title: "Check"}).Run(host)
	if len(ran) != 1 || ran[0] != api.UpdateOpCheck || len(*host.toasts) != 0 {
		t.Fatalf("ran %v, toasts %v; want the check run without a toast", ran, *host.toasts)
	}
}

// TestSwitchingTheChannelRefreshesThePageAtOnce: the Updates page shows the new channel right after Enter.
func TestSwitchingTheChannelRefreshesThePageAtOnce(t *testing.T) {
	deps := Deps{RunUpdate: func(string, string) error { return nil }}
	host := &refreshRecorder{toastRecorder: toastRecorder{toasts: new([]string)}}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Title: "Switch"}).Run(host)
	if host.refreshes != 1 || host.pops != 0 {
		t.Fatalf("refreshes = %d, pops = %d; want the page re-read once and kept", host.refreshes, host.pops)
	}
}

type refreshRecorder struct {
	toastRecorder
	refreshes int
	pops      int
}

func (h *refreshRecorder) Refresh() { h.refreshes++ }

func (h *refreshRecorder) Pop() { h.pops++ }

// TestChoosingAChannelGoesBackToTheUpdatesPage: a channel picked from the channel list returns to the page below it.
func TestChoosingAChannelGoesBackToTheUpdatesPage(t *testing.T) {
	var chosen string
	deps := Deps{RunUpdate: func(op, target string) error { chosen = op + ":" + target; return nil }}
	host := &refreshRecorder{toastRecorder: toastRecorder{toasts: new([]string)}}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Target: api.UpdateChannelDev, Title: "Choose"}).Run(host)
	if chosen != "channel:dev" || host.pops != 1 || host.refreshes != 1 {
		t.Fatalf("chosen %q, pops %d, refreshes %d; want dev chosen, one pop and one refresh", chosen, host.pops, host.refreshes)
	}
}

func TestRefusedUpdateActionShowsItsReasonAsAToast(t *testing.T) {
	deps := Deps{RunUpdate: func(string, string) error {
		return errors.New("restart to finish the update before switching channels")
	}}
	host := &toastRecorder{toasts: new([]string)}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Title: "Switch"}).Run(host)
	if len(*host.toasts) != 1 || (*host.toasts)[0] != "restart to finish the update before switching channels" {
		t.Fatalf("toasts = %v, want the refusal", *host.toasts)
	}
}

func TestUpdateActionWithoutAnUpdaterIsUnavailable(t *testing.T) {
	host := &toastRecorder{toasts: new([]string)}
	ActionFor(Deps{}, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpInstall, Title: "Restart"}).Run(host)
	if len(*host.toasts) != 1 || (*host.toasts)[0] != unavailableMessage {
		t.Fatalf("toasts = %v, want the unavailable message", *host.toasts)
	}
}
