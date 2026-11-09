package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestIndicatorStatusMapsExtensionStatusesOntoThePanelTones(t *testing.T) {
	cases := []struct {
		status api.IndicatorStatus
		want   commands.Tone
	}{
		{api.IndicatorRecording, commands.ToneLoading},
		{api.IndicatorTranscribing, commands.ToneLoading},
		{"", commands.ToneLoading},
		{api.IndicatorSuccess, commands.ToneSuccess},
		{api.IndicatorError, commands.ToneError},
	}
	for _, c := range cases {
		if got := indicatorStatusOf(api.IndicatorInput{Title: "Work", Status: c.status}).Tone; got != c.want {
			t.Fatalf("status %q has tone %q; want %q", c.status, got, c.want)
		}
	}
	for _, tc := range []struct {
		input api.IndicatorStatus
		want  commands.Activity
	}{{api.IndicatorRecording, commands.ActivityRecording}, {api.IndicatorTranscribing, commands.ActivityTranscribing}, {api.IndicatorLoading, commands.ActivityLoading}} {
		if got := indicatorStatusOf(api.IndicatorInput{Title: "Localized", Status: tc.input}).Activity; got != tc.want {
			t.Errorf("activity for %q = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIndicatorProgressJoinsTheSubtitle(t *testing.T) {
	if got := indicatorStatusOf(api.IndicatorInput{Title: "Converting", Value: 3, Total: 10}).Subtitle; got != "3 of 10" {
		t.Fatalf("subtitle = %q; want the progress alone", got)
	}
	if got := indicatorStatusOf(api.IndicatorInput{Title: "Converting", Subtitle: "clip.mov", Value: 3, Total: 10}).Subtitle; got != "clip.mov · 3 of 10" {
		t.Fatalf("subtitle = %q; want the subtitle then the progress", got)
	}
}

func TestDictationLevelDeliveryCoalescesAndDropsObsoleteUpdates(t *testing.T) {
	queue := make(completionUIQueue, 8)
	p := &App{uiQueue: queue}
	var got []float64
	s := &extensionIndicators{app: p, shown: true, recording: true, gen: 1,
		applyLevel: func(level float64) { got = append(got, level) }}
	s.reportDictationLevel(0.2, func() bool { return true })
	s.reportDictationLevel(0.6, func() bool { return true })
	if len(queue) != 1 {
		t.Fatalf("queued %d callbacks, want one", len(queue))
	}
	(<-queue)()
	if len(got) != 1 || got[0] != 0.6 {
		t.Fatalf("delivered %v", got)
	}
	s.reportDictationLevel(0.8, func() bool { return true })
	s.mu.Lock()
	s.gen++
	s.mu.Unlock()
	s.reportDictationLevel(0.4, func() bool { return true })
	(<-queue)()
	(<-queue)()
	if len(got) != 2 || got[1] != 0.4 {
		t.Fatalf("after replacement delivered %v", got)
	}
	s.reportDictationLevel(1, func() bool { return true })
	s.mu.Lock()
	s.shown = false
	s.gen++
	s.mu.Unlock()
	(<-queue)()
	if len(got) != 2 {
		t.Fatalf("hidden indicator received %v", got)
	}
}

func TestDictationLevelDropsExpiredOperationAcrossReplacement(t *testing.T) {
	queue := make(completionUIQueue, 8)
	p := &App{uiQueue: queue}
	var got []float64
	s := &extensionIndicators{app: p, shown: true, recording: true, gen: 1,
		applyLevel: func(level float64) { got = append(got, level) }}
	oldCurrent := true
	s.reportDictationLevel(0.8, func() bool { return oldCurrent })
	oldCurrent = false
	s.reportDictationLevel(0.4, func() bool { return true })
	// A delayed old callback must not replace the newer operation's sample.
	s.reportDictationLevel(0.9, func() bool { return oldCurrent })
	(<-queue)()
	if len(got) != 1 || got[0] != 0.4 {
		t.Fatalf("delivered %v, want only the current operation", got)
	}
	s.reportDictationLevel(0.7, func() bool { return oldCurrent })
	if len(queue) != 0 {
		t.Fatal("expired operation queued an update")
	}
}

func TestDictationLevelDoesNotReachReplacementIndicator(t *testing.T) {
	queue := make(completionUIQueue, 8)
	p := &App{uiQueue: queue}
	var got []float64
	s := &extensionIndicators{app: p, shown: true, recording: true, owner: "nevermind.dictation", id: "dictation", gen: 1,
		applyLevel: func(level float64) { got = append(got, level) }}
	s.reportDictationLevel(0.8, func() bool { return true })
	s.ShowIndicator("other.extension", api.IndicatorInput{ID: "other", Status: api.IndicatorRecording})
	(<-queue)()
	s.reportDictationLevel(0.5, func() bool { return true })
	if len(got) != 0 || len(queue) != 1 {
		t.Fatalf("replacement received levels %v, queued callbacks %d", got, len(queue))
	}
}
