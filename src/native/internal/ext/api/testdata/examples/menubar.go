// Example: a menu bar status item for a pomodoro timer. The title changes every second, which is
// why it lives in the title and not in the rows: changing rows rebuilds the native menu.
package main

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type pomodoro struct{}

type timerState struct {
	RunningSinceMs int64 `json:"runningSinceMs"`
	RemainingMs    int64 `json:"remainingMs"`
}

var (
	_ api.Extension       = pomodoro{}
	_ api.MenuBarProvider = pomodoro{}
	_ api.ActionHandler   = pomodoro{}
)

func (pomodoro) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: "acme.pomodoro", Title: "Pomodoro"},
	}
}

func (pomodoro) MenuBarTriggers() []api.Trigger {
	return []api.Trigger{{Type: api.TriggerInterval, Every: "1s"}}
}

func (pomodoro) MenuBars(ctx api.Context) ([]api.MenuBar, error) {
	state, err := loadTimer(ctx)
	if err != nil {
		return nil, err
	}
	label := "Start"
	if state.RunningSinceMs != 0 {
		label = "Pause"
	}
	toggle := api.Run("toggle", label, nil)
	return []api.MenuBar{{
		ID:      "main",
		Icon:    "timer",
		Title:   formatRemaining(state.remaining(ctx.Clock().Now())),
		Tooltip: "Pomodoro",
		Sections: []api.MenuSection{{Rows: []api.MenuRow{
			{ID: "toggle", Title: label, Action: &toggle},
		}}},
	}}, nil
}

func (state timerState) remaining(now time.Time) time.Duration {
	left := time.Duration(state.RemainingMs) * time.Millisecond
	if state.RunningSinceMs != 0 {
		left -= now.Sub(time.UnixMilli(state.RunningSinceMs))
	}
	return max(left, 0)
}

func formatRemaining(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func loadTimer(ctx api.Context) (timerState, error) {
	state := timerState{RemainingMs: int64(25 * time.Minute / time.Millisecond)}
	if _, err := ctx.Storage().Get("timer", &state); err != nil {
		return timerState{}, err
	}
	return state, nil
}

func (pomodoro) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "toggle" {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	state, err := loadTimer(ctx)
	if err != nil {
		return api.Result{}, err
	}
	now := ctx.Clock().Now()
	if state.RunningSinceMs != 0 {
		state.RemainingMs = state.remaining(now).Milliseconds()
		state.RunningSinceMs = 0
	} else {
		state.RunningSinceMs = now.UnixMilli()
	}
	if err := ctx.Storage().Set("timer", state); err != nil {
		return api.Result{}, err
	}
	ctx.Views().RefreshMenuBar()
	return api.Result{}, nil
}
