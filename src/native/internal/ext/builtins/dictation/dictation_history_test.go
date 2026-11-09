package dictation

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
)

func TestHistoryListsUnfinishedDictationsFirstAndRetryRecoversThem(t *testing.T) {
	ctx := newContext()
	ctx.dictation.recordings = []api.SavedDictation{{ID: "dictation-rec", CreatedAtMs: testNow.UnixMilli(), SegmentCount: 2}}
	ctx.dictation.retryText = "recovered words"
	ext := New(Legacy{History: []history.Entry{{ID: "old", Text: "older", CreatedAt: testNow.Add(-time.Hour).UnixMilli()}}})

	view := run(t, ext, ctx, HistoryCommandID, nil, nil).View
	if view == nil || len(view.Items) != 2 || view.Items[0].ID != "dictation-rec" || view.Items[0].Title != recordingTitle {
		t.Fatalf("history view %+v; want the unfinished dictation first", view)
	}
	result := run(t, ext, ctx, refRetry, map[string]string{argID: "dictation-rec"}, nil)
	if len(ctx.dictation.retried) != 1 || ctx.dictation.retried[0] != "dictation-rec" {
		t.Fatalf("retried %v; want the saved dictation", ctx.dictation.retried)
	}
	if len(ctx.dictation.deleted) != 1 || ctx.dictation.deleted[0] != "dictation-rec" {
		t.Fatalf("deleted %v; want the audio dropped after the text is saved", ctx.dictation.deleted)
	}
	if result.Toast == nil || result.Toast.Message != "Transcription recovered" || result.Toast.Tone != "success" {
		t.Fatalf("toast %+v; want the recovery toast", result.Toast)
	}
	if entries := storedHistory(t, ctx); len(entries) != 2 || entries[0].Text != "recovered words" {
		t.Fatalf("history %+v; want the recovered text in front", entries)
	}
}

func TestAnEmptyRetryKeepsTheRecording(t *testing.T) {
	ctx := newContext()
	ctx.dictation.retryText = "  "
	result := run(t, New(Legacy{}), ctx, refRetry, map[string]string{argID: "rec"}, nil)
	if len(ctx.dictation.deleted) != 0 {
		t.Fatal("an empty retry dropped the audio")
	}
	if result.Toast == nil || result.Toast.Message != "No speech detected" || result.Toast.Tone != "error" {
		t.Fatalf("toast %+v; want No speech detected as an error", result.Toast)
	}
}

func TestDeleteTranscriptRemovesOnlyThatEntry(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	saveAll(t, ctx, history.Entry{ID: "a", Text: "first", CreatedAt: 1}, history.Entry{ID: "b", Text: "second", CreatedAt: 2})
	run(t, ext, ctx, refDeleteTranscript, map[string]string{argID: "a"}, nil)
	entries := storedHistory(t, ctx)
	if len(entries) != 1 || entries[0].ID != "b" {
		t.Fatalf("history %+v; want only the second transcript", entries)
	}
}

func TestClearHistoryRemovesTranscriptsAndRecordings(t *testing.T) {
	ctx := newContext()
	ctx.dictation.recordings = []api.SavedDictation{{ID: "rec-1", CreatedAtMs: testNow.UnixMilli()}}
	saveAll(t, ctx, history.Entry{ID: "a", Text: "first", CreatedAt: testNow.UnixMilli()})
	result := run(t, New(Legacy{}), ctx, refClearHistory, nil, nil)
	if len(storedHistory(t, ctx)) != 0 || len(ctx.dictation.deleted) != 1 {
		t.Fatalf("history %+v deleted %v; want both cleared", storedHistory(t, ctx), ctx.dictation.deleted)
	}
	if result.Toast == nil || result.Toast.Message != "Dictation history cleared" {
		t.Fatalf("toast %+v; want the cleared toast", result.Toast)
	}
}

func TestHistoryDropsTheOldestEntriesPastTheCap(t *testing.T) {
	ctx := newContext()
	var entries []history.Entry
	for i := 0; i < history.MaxEntries+5; i++ {
		entries = append(entries, history.Entry{ID: fmt.Sprintf("e%d", i), Text: "text", CreatedAt: int64(i + 1)})
	}
	ctx.storage.values[storageHistoryKey] = mustJSON(t, entries)
	view := run(t, New(Legacy{}), ctx, HistoryCommandID, nil, nil).View
	if len(view.Items) != history.MaxEntries {
		t.Fatalf("items = %d; want the cap of %d", len(view.Items), history.MaxEntries)
	}
}

func TestSettingsFormSavesTheChoicesAndKeepsCleaningOffWithoutAnAccount(t *testing.T) {
	ctx := newContext()
	ctx.ai.available = false
	ext := New(Legacy{})
	view := run(t, ext, ctx, RefOpenSettings, nil, nil).View
	if view == nil || view.Form == nil || len(view.Form.Fields) != 5 {
		t.Fatalf("settings view %+v; want the five fields", view)
	}
	form := api.FormValues{
		fieldDevice:     api.TextValue("mic-1"),
		fieldCleanup:    api.FlagValue(true),
		fieldDictionary: api.TextValue("Kubernetes"),
		fieldClipboard:  api.FlagValue(true),
		fieldScreen:     api.FlagValue(true),
	}
	if result := run(t, ext, ctx, refSaveSettings, nil, form); result.Navigation != api.NavPop {
		t.Fatalf("navigation %q; want pop", result.Navigation)
	}
	settings := ext.readSettings(ctx)
	if settings.DeviceID != "mic-1" || settings.CleanupWithAI || settings.Dictionary != "Kubernetes" || !settings.CopyToClipboard || !settings.UseScreenContext {
		t.Fatalf("settings %+v; want the saved choices, cleaning off without an account", settings)
	}
}

func TestLegacyChoicesAnswerUntilTheBuiltInSaves(t *testing.T) {
	ctx := newContext()
	legacy := Settings{DeviceID: "legacy-mic"}
	ext := New(Legacy{Settings: &legacy, History: []history.Entry{{ID: "old", Text: "kept", CreatedAt: 9}}})
	if got := ext.readSettings(ctx); got.DeviceID != "legacy-mic" {
		t.Fatalf("settings %+v; want the legacy choices", got)
	}
	if entries := ext.readHistory(ctx); len(entries) != 1 || entries[0].Text != "kept" {
		t.Fatalf("history %+v; want the legacy transcript", entries)
	}
	ctx.storage.values[storageSettingsKey] = mustJSON(t, Settings{DeviceID: "saved"})
	if got := ext.readSettings(ctx); got.DeviceID != "saved" {
		t.Fatalf("settings %+v; want the saved choices to win", got)
	}
}

func TestCommandsKeepTheOneXShortcutIDsAndRunInTheBackground(t *testing.T) {
	commands, err := Extension{}.Commands(newContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0].ID != DictateCommandID || commands[1].ID != HistoryCommandID {
		t.Fatalf("commands %+v; want Dictate and Dictation History", commands)
	}
	if !commands[0].Customizable || commands[0].Mode != api.ModeBackground || !commands[0].DismissAfterRun {
		t.Fatalf("Dictate %+v; want a customizable background command that dismisses", commands[0])
	}
	if !commands[1].Customizable {
		t.Fatalf("Dictation History %+v; want it customizable", commands[1])
	}
}

func TestManifestDeclaresWhatTheActionsUse(t *testing.T) {
	manifest := Extension{}.Manifest()
	if manifest.Identity.ID != ID {
		t.Fatalf("id %q", manifest.Identity.ID)
	}
	for _, want := range []api.Capability{api.CapabilityDictation, api.CapabilityAI, api.CapabilityOCR, api.CapabilityDesktopApps} {
		if !hasCapability(manifest, want) {
			t.Fatalf("manifest misses %s", want)
		}
	}
}

func TestAnUnknownActionIsRefused(t *testing.T) {
	_, err := New(Legacy{}).RunAction(newContext(), api.Invocation{Ref: api.ActionRef{ID: "nope"}})
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err %v; want invalid input", err)
	}
}

func hasCapability(manifest api.Manifest, want api.Capability) bool {
	for _, capability := range manifest.Capabilities {
		if capability == want {
			return true
		}
	}
	return false
}

// storedHistory returns the transcripts the built-in saved.
