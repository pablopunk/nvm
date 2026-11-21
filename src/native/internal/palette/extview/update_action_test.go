package extview

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestUpdateActionRunsItsOperationOnTheHost(t *testing.T) {
	var ran []string
	deps := Deps{RunUpdate: func(op string) error { ran = append(ran, op); return nil }}
	host := &refreshRecorder{toastRecorder: toastRecorder{toasts: new([]string)}}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpCheck, Title: "Check"}).Run(host)
	if len(ran) != 1 || ran[0] != api.UpdateOpCheck || len(*host.toasts) != 0 {
		t.Fatalf("ran %v, toasts %v; want the check run without a toast", ran, *host.toasts)
	}
}

// TestSwitchingTheChannelRefreshesThePageAtOnce: the Updates page shows the new channel right after Enter.
func TestSwitchingTheChannelRefreshesThePageAtOnce(t *testing.T) {
	deps := Deps{RunUpdate: func(string) error { return nil }}
	host := &refreshRecorder{toastRecorder: toastRecorder{toasts: new([]string)}}
	ActionFor(deps, "nevermind.updates", api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Title: "Switch"}).Run(host)
	if host.refreshes != 1 {
		t.Fatalf("refreshes = %d, want the page re-read once", host.refreshes)
	}
}

type refreshRecorder struct {
	toastRecorder
	refreshes int
}

func (h *refreshRecorder) Refresh() { h.refreshes++ }

func TestRefusedUpdateActionShowsItsReasonAsAToast(t *testing.T) {
	deps := Deps{RunUpdate: func(string) error { return errors.New("restart to finish the update before switching channels") }}
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
