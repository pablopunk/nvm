package dictation

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/cleanup"
)

func TestDictateStartsWithTheChosenMicrophoneAndMutesTheOutput(t *testing.T) {
	ctx := newContext()
	ctx.storage.values[storageSettingsKey] = []byte(`{"deviceId":"mic-1"}`)
	dictate(t, New(Legacy{}), ctx)
	if len(ctx.dictation.starts) != 1 {
		t.Fatalf("starts = %d; want one", len(ctx.dictation.starts))
	}
	if got := ctx.dictation.starts[0]; got.DeviceID != "mic-1" || !got.MuteSystemAudioWhileRecord {
		t.Fatalf("start options = %+v; want mic-1 with the output muted", got)
	}
	if last := ctx.indicators.last(); last.Subtitle != "Listening" || last.Status != api.IndicatorRecording {
		t.Fatalf("indicator = %+v; want Listening", last)
	}
}

func TestDictateStopPastesTheTranscriptIntoTheAppThatHadFocus(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "hello there"
	paste := pasted(t, dictate(t, ext, ctx))
	if paste.Text != "hello there" || !paste.DismissAfterRun {
		t.Fatalf("paste = %+v; want the transcript, dismissing the palette", paste)
	}
	if paste.Paste == nil || !paste.Paste.Concealed || !paste.Paste.RestoreClipboard || paste.Paste.ExpectedFrontmostID != "com.apple.Notes" {
		t.Fatalf("paste options = %+v; want concealed, clipboard restored, aimed at the app that had focus", paste.Paste)
	}
	if entries := storedHistory(t, ctx); len(entries) != 1 || entries[0].Text != "hello there" {
		t.Fatalf("history = %+v; want the transcript saved", entries)
	}
}

func TestCopyToClipboardKeepsTheTranscriptOnTheClipboard(t *testing.T) {
	ctx := newContext()
	ctx.storage.values[storageSettingsKey] = []byte(`{"deviceId":"default","copyToClipboard":true}`)
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "keep me"
	paste := pasted(t, dictate(t, ext, ctx))
	if paste.Paste.Concealed || paste.Paste.RestoreClipboard {
		t.Fatalf("paste options = %+v; want the clipboard kept", paste.Paste)
	}
}

func TestSilentDictationSaysNoSpeechAndPastesNothing(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "   "
	result := dictate(t, ext, ctx)
	if result.Next != nil {
		t.Fatalf("result %+v; want no paste", result)
	}
	if last := ctx.indicators.last(); last.Subtitle != "No speech detected" || last.DurationMs != noSpeechDuration.Milliseconds() {
		t.Fatalf("indicator = %+v; want No speech detected for %v", last, noSpeechDuration)
	}
	if len(storedHistory(t, ctx)) != 0 {
		t.Fatal("a silent dictation reached the history")
	}
}

func TestDictateWithoutAnAccountShowsTheSignInLine(t *testing.T) {
	ctx := newContext()
	ctx.dictation.available = false
	dictate(t, New(Legacy{}), ctx)
	if len(ctx.dictation.starts) != 0 {
		t.Fatal("a dictation started without an account")
	}
	if last := ctx.indicators.last(); last.Status != api.IndicatorError || last.Subtitle != "Dictation unavailable: Sign in to use cloud dictation" {
		t.Fatalf("indicator = %+v; want the sign-in line as an error", last)
	}
}

func TestAMicrophoneFailureShowsItsWords(t *testing.T) {
	ctx := newContext()
	ctx.dictation.startErr = errors.New("The microphone is turned off")
	dictate(t, New(Legacy{}), ctx)
	if last := ctx.indicators.last(); last.Status != api.IndicatorError || last.Subtitle != "Dictation unavailable: The microphone is turned off" || last.DurationMs != errorDuration.Milliseconds() {
		t.Fatalf("indicator = %+v; want the failure words for %v", last, errorDuration)
	}
}

func TestACancelledStopClosesTheIndicatorWithoutWords(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.stopErr = errors.New("")
	result := dictate(t, ext, ctx)
	if result.Next != nil || result.Toast != nil {
		t.Fatalf("result %+v; want nothing said", result)
	}
	if ctx.indicators.changes[len(ctx.indicators.changes)-1] != "hide" {
		t.Fatalf("changes %v; want the indicator closed last", ctx.indicators.changes)
	}
}

func TestAKeyPressedWhileTheMicrophoneOpensDoesNothing(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	ext.session.beginStart("")
	result := dictate(t, ext, ctx)
	if result.Next != nil || ctx.dictation.stops != 0 || len(ctx.dictation.starts) != 0 {
		t.Fatalf("result %+v; a press while starting must do nothing", result)
	}
}

func TestAKeyPressedWhileTranscribingDoesNothing(t *testing.T) {
	ctx := newContext()
	ctx.dictation.status = api.DictationTranscribing
	dictate(t, New(Legacy{}), ctx)
	if ctx.dictation.stops != 0 || len(ctx.dictation.starts) != 0 {
		t.Fatal("a press while transcribing reached the recorder")
	}
}

func TestCleanupAsksTheFastModelAndPastesItsAnswer(t *testing.T) {
	ctx := newContext()
	ctx.storage.values[storageSettingsKey] = []byte(`{"cleanupWithAi":true,"dictionary":"Grafana"}`)
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "the graphana dashboard"
	paste := pasted(t, dictate(t, ext, ctx))
	if paste.Text != "Cleaned." {
		t.Fatalf("paste text %q; want the cleaned text", paste.Text)
	}
	option := ctx.ai.options[len(ctx.ai.options)-1]
	if option.Model != api.ModelFast || option.System != cleanup.System {
		t.Fatalf("ai options %+v; want the fast tier with the cleanup system prompt", option)
	}
	if !strings.Contains(ctx.ai.prompts[len(ctx.ai.prompts)-1], "Preferred terms and spellings") || !strings.Contains(ctx.ai.prompts[len(ctx.ai.prompts)-1], "Grafana") {
		t.Fatalf("prompt %q; want the dictionary term", ctx.ai.prompts[len(ctx.ai.prompts)-1])
	}
}

func TestACleanupThatFailsPastesTheRawTranscript(t *testing.T) {
	ctx := newContext()
	ctx.storage.values[storageSettingsKey] = []byte(`{"cleanupWithAi":true}`)
	ctx.ai.err = errors.New("backend down")
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "raw words"
	if paste := pasted(t, dictate(t, ext, ctx)); paste.Text != "raw words" {
		t.Fatalf("paste text %q; want the raw transcript", paste.Text)
	}
	if len(ctx.logs.warnings) == 0 {
		t.Fatal("a failed cleanup was not logged")
	}
}

func TestScreenWordsJoinTheCleanupPrompt(t *testing.T) {
	ctx := newContext()
	ctx.ocr = fakeOCR{text: "Kubernetes dashboard the Kubernetes"}
	ctx.storage.values[storageSettingsKey] = []byte(`{"cleanupWithAi":true,"useScreenContext":true}`)
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "open the dashboard"
	dictate(t, ext, ctx)
	prompt := ctx.ai.prompts[len(ctx.ai.prompts)-1]
	if !strings.Contains(prompt, "Screen terms") || !strings.Contains(prompt, "Kubernetes") {
		t.Fatalf("prompt %q; want the screen words", prompt)
	}
}

func TestScreenWordsAreNotReadUnlessTheSettingsAskForThem(t *testing.T) {
	ctx := newContext()
	ctx.ocr = fakeOCR{err: errors.New("screen is not available")}
	ctx.storage.values[storageSettingsKey] = []byte(`{"cleanupWithAi":true}`)
	ext := New(Legacy{})
	dictate(t, ext, ctx)
	ctx.dictation.transcript = "words"
	if paste := pasted(t, dictate(t, ext, ctx)); paste.Text != "Cleaned." {
		t.Fatalf("paste text %q; the screen must not be needed", paste.Text)
	}
}
