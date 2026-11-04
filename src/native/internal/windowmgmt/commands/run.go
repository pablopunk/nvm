package commands

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// Runner runs commands through the engine with the settings the user chose. Its methods return the message
// to tell the user, empty when the window reached its frame and nothing needs saying.
type Runner struct {
	Engine   *windowmgmt.Service
	Settings func() geometry.Settings
}

// Execute runs cmd on the target of inv: the focused window of the app inv names.
func (r Runner) Execute(ctx context.Context, cmd Command, inv windowmgmt.Invocation) string {
	if cmd.IsFocus() {
		return messageOf(r.Engine.FocusNeighbor(ctx, cmd.direction, geometry.NeighborOptions{}, inv))
	}
	settings := r.Settings()
	layout, ok := geometry.LayoutSpecFor(cmd.preset, settings)
	if !ok {
		return ""
	}
	return r.ExecuteLayout(ctx, layout, inv)
}

// ExecuteLayout places the target of inv with an explicit layout, such as an exact move or resize, using the user's
// gap settings.
func (r Runner) ExecuteLayout(ctx context.Context, layout geometry.Layout, inv windowmgmt.Invocation) string {
	settings := r.Settings()
	result, err := r.Engine.Apply(ctx, layout, geometry.OptionsFromSettings(settings), inv)
	return outcomeMessage(result, err)
}

// ExecuteOnWindow runs a layout command on the window a handle from the window list names. Focus commands
// have no window of their own and return no message.
func (r Runner) ExecuteOnWindow(ctx context.Context, cmd Command, handle string) string {
	if cmd.IsFocus() {
		return ""
	}
	settings := r.Settings()
	layout, ok := geometry.LayoutSpecFor(cmd.preset, settings)
	if !ok {
		return ""
	}
	result, err := r.Engine.ApplyToWindow(ctx, handle, layout, geometry.OptionsFromSettings(settings))
	return outcomeMessage(result, err)
}

func outcomeMessage(result windowmgmt.Result, err error) string {
	if err != nil {
		return err.Error()
	}
	if result.Outcome == geometry.Partial {
		return result.Message
	}
	return ""
}

func messageOf(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
