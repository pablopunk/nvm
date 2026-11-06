package design

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type indicatorEntryKey struct{ sequence uint64 }

// IndicatorEntrance plays once when a hidden indicator is shown. Status updates retain the same sequence, so they
// never restart entry. MyGo owns frame scheduling and completes keyframes immediately when reduced motion is enabled.
func IndicatorEntrance(c *ui.Context, sequence uint64, body func()) {
	panel := ui.Column(c).Key(indicatorEntryKey{sequence}).Fill()
	if sequence != 0 {
		progress := panel.Keyframes("entry", []ui.Keyframe{
			{Value: 0},
			{At: 180 * time.Millisecond, Value: 1, Ease: ui.EaseOut},
		})
		panel.Opacity(progress)
		if !c.Preferences().ReduceMotion {
			panel.Translate(0, 4*(1-progress))
		}
	}
	panel.Children(body)
}

// IndicatorContentTransition crossfades status copy without moving the panel or its neighbours.
func IndicatorContentTransition() ui.ContentTransition {
	return ui.ContentTransition{Animation: &ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut}}
}

// IndicatorActivity returns a looping phase only while a live activity indicator is built.
// Rebuilding with another activity key starts the new phase at its own visual origin.
func IndicatorActivity(c *ui.Context, activity string, reducedMotion bool) float32 {
	if reducedMotion || activity == "" {
		return 0
	}
	return ui.Box(c).Key(activity).Absolute().Size(0, 0).Keyframes("ambient-activity", []ui.Keyframe{
		{Value: 0},
		{At: 1600 * time.Millisecond, Value: 1, Ease: ui.Linear},
	}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
}
