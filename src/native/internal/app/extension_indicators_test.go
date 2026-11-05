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
