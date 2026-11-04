package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCleanupPastesTheModelsTextAndShowsTheCleaningIndicator(t *testing.T) {
	h := newDictationHarness(t)
	h.settings.CleanupWithAI = true
	h.completer = func(context.Context, string, string) (string, error) { return "Cleaned text.", nil }
	h.speak("raw text")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.pastes) != 1 || h.pastes[0].text != "Cleaned text." {
		t.Fatalf("pastes = %+v, want the cleaned text pasted", h.pastes)
	}
	if !strings.Contains(strings.Join(h.panel.eventList(), " "), "show:cleaning") {
		t.Fatalf("indicator events = %v, want the cleaning state", h.panel.eventList())
	}
}

func TestCleanupIsOffUnlessTheSettingsAskForIt(t *testing.T) {
	h := newDictationHarness(t)
	called := false
	h.completer = func(context.Context, string, string) (string, error) {
		called = true
		return "never", nil
	}
	h.speak("plain")
	h.flow.Toggle()
	h.flow.Toggle()

	if called || len(h.pastes) != 1 || h.pastes[0].text != "plain" {
		t.Fatalf("cleanup called = %v, pastes = %+v; want the raw transcript and no model call", called, h.pastes)
	}
}

func TestAFailedCleanupPastesTheTranscriptAndLogsWhy(t *testing.T) {
	h := newDictationHarness(t)
	h.settings.CleanupWithAI = true
	h.completer = func(context.Context, string, string) (string, error) { return "", errors.New("route paused") }
	h.speak("kept as said")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.pastes) != 1 || h.pastes[0].text != "kept as said" {
		t.Fatalf("pastes = %+v, want the transcript pasted", h.pastes)
	}
	if !containsString(h.warned, "dictation.cleanup-failed") {
		t.Fatalf("warnings = %v, want the cleanup failure logged", h.warned)
	}
}

func TestTheDictionaryReachesTheCleanupPrompt(t *testing.T) {
	h := newDictationHarness(t)
	h.settings.CleanupWithAI = true
	h.settings.Dictionary = "Nevermind\nFigma"
	var prompt string
	h.completer = func(_ context.Context, _, user string) (string, error) {
		prompt = user
		return "ok", nil
	}
	h.speak("ok")
	h.flow.Toggle()
	h.flow.Toggle()

	if !strings.Contains(prompt, "Nevermind\nFigma") {
		t.Fatalf("prompt = %q, want the dictionary terms", prompt)
	}
}
