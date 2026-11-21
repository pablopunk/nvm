// Package timers is the built-in countdown. "timer 5m" at the root offers to start one, Start Timer and Cancel Timer are
// commands, and a running timer shows its time left as a menu bar item. When the time is up, a notification says so.
//
// Layer: ext. The package imports api, search and the standard library.
package timers

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.timers"

// Command IDs. Cancel Timer is listed only while a timer runs.
const (
	startCommandID  = "start"
	cancelCommandID = "cancel"
)

// Refs of the actions the root row and the scheduled run run. The args carry the timer's duration.
const (
	refStartDuration = "start-duration"
	refFinish        = "finish"
	argDuration      = "duration"
)

// timerKey is the key the one timer is scheduled under. Starting a timer replaces the one before it.
const timerKey = "timer"

// scoreTimer is the root row's score: a timer query is answered above the commands and the fallbacks.
const scoreTimer = 990

// menuBarID names the menu bar item the countdown shows in.
const menuBarID = "timer"

// tickEvery is how often the menu bar countdown renders, so its time left is a second behind at most.
const tickEvery = "1s"

// Extension is the timers built-in. It keeps no state of its own: the host's one-shot scheduler holds the running timer.
type Extension struct{}

// New returns the timers built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. It notifies when a timer is up, which the system declaration covers.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Timers"},
		Capabilities: []api.Capability{api.CapabilitySystem},
	}
}

// Commands lists Start Timer, and Cancel Timer while a timer runs. Both are found by a typed query only.
func (Extension) Commands(ctx api.Context) ([]api.Command, error) {
	commands := []api.Command{{
		ID: startCommandID, Title: "Start Timer", Subtitle: "Type e.g. “timer 5m”", Icon: "timer",
		Keywords: []string{"timer", "countdown", "alarm"}, Appearance: &api.Appearance{Foreground: api.ColorOrange},
		Placement: []api.Placement{api.PlacementSearch}, Section: search.KindCommands, LearnKey: "cmd:Start Timer",
	}}
	_, running, err := ctx.Scheduler().Pending(timerKey)
	if err != nil {
		return nil, err
	}
	if running {
		commands = append(commands, api.Command{
			ID: cancelCommandID, Title: "Cancel Timer", Icon: "timer",
			Keywords: []string{"timer", "cancel", "stop"}, Appearance: &api.Appearance{Foreground: api.ColorOrange},
			Placement: []api.Placement{api.PlacementSearch}, Section: search.KindCommands, LearnKey: "cmd:Cancel Timer",
			DismissAfterRun: true,
		})
	}
	return commands, nil
}

// SearchItems offers to start a countdown for a query such as "timer 5m".
func (Extension) SearchItems(_ api.Context, query string) ([]api.Item, error) {
	duration, ok := parseTimer(query)
	if !ok || duration <= 0 {
		return nil, nil
	}
	start := api.Run(refStartDuration, "Start", map[string]string{argDuration: duration.String()})
	start.DismissAfterRun = true
	return []api.Item{{
		Title: "Start timer: " + duration.String(), Subtitle: "Shows a countdown in the tray", Icon: "timer",
		Appearance: &api.Appearance{Foreground: api.ColorOrange}, Section: search.KindTimer, Score: scoreTimer,
		Primary: &start,
	}}, nil
}

// MenuBarTriggers re-renders the countdown every second while the palette's host runs.
func (Extension) MenuBarTriggers() []api.Trigger {
	return []api.Trigger{{Type: api.TriggerInterval, Every: tickEvery}}
}

// MenuBars shows the time left of the running timer, and nothing while no timer runs or the time is up.
func (Extension) MenuBars(ctx api.Context) ([]api.MenuBar, error) {
	due, running, err := ctx.Scheduler().Pending(timerKey)
	if err != nil || !running {
		return nil, err
	}
	left := due.Sub(ctx.Clock().Now())
	if left <= 0 {
		return nil, nil
	}
	return []api.MenuBar{{ID: menuBarID, Icon: "timer", Title: countdown(left)}}, nil
}

// RunAction starts, cancels and finishes the timer, and runs the Start Timer command.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case startCommandID:
		query := api.SetQuery("Start Timer", "timer 5m")
		return api.Result{Next: &query}, nil
	case refStartDuration:
		return api.Result{}, startTimer(ctx, in.Ref.Args[argDuration])
	case cancelCommandID:
		_, err := ctx.Scheduler().Cancel(timerKey)
		ctx.Views().RefreshMenuBar()
		return api.Result{}, err
	case refFinish:
		finish(ctx, in.Ref.Args[argDuration])
		return api.Result{}, nil
	}
	return api.Result{}, fmt.Errorf("the timers built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// startTimer schedules the finish of a timer of the duration, replacing a timer that runs.
func startTimer(ctx api.Context, duration string) error {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("a timer needs a duration: %w", api.ErrInvalidInput)
	}
	ref := api.ActionRef{ID: refFinish, Args: map[string]string{argDuration: d.String()}}
	ctx.Views().RefreshMenuBar()
	return ctx.Scheduler().Once(timerKey, ctx.Clock().Now().Add(d), ref)
}

// finish tells the user the timer is up and takes the countdown off the menu bar. A notification the system refuses is
// not an error: the menu bar change still shows the timer ended.
func finish(ctx api.Context, duration string) {
	_ = ctx.System().Notify("Timer finished", duration+" is up")
	ctx.Views().RefreshMenuBar()
}

// countdown is the time left as minutes and seconds, such as "4:59".
func countdown(left time.Duration) string {
	return fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
}
