package dictation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/cleanup"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
)

var testNow = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

// fakeDictation is the host's recorder: it answers from a script and records each call.
type fakeDictation struct {
	api.Dictation
	status       api.DictationStatus
	available    bool
	devices      []api.DictationDevice
	startErr     error
	starts       []api.DictationStartOptions
	transcript   string
	stopErr      error
	stops        int
	recordings   []api.SavedDictation
	retried      []string
	retryText    string
	retryErr     error
	deleted      []string
	deleteFailed bool
}

func (d *fakeDictation) Status() (api.DictationStatus, error) { return d.status, nil }
func (d *fakeDictation) APIAvailable() (bool, error)          { return d.available, nil }
func (d *fakeDictation) Devices() ([]api.DictationDevice, error) {
	return d.devices, nil
}

func (d *fakeDictation) Start(options api.DictationStartOptions) error {
	d.starts = append(d.starts, options)
	if d.startErr == nil {
		d.status = api.DictationRecording
	}
	return d.startErr
}

func (d *fakeDictation) Stop() (string, error) {
	d.stops++
	d.status = api.DictationIdle
	return d.transcript, d.stopErr
}

func (d *fakeDictation) Cancel() error { return nil }

func (d *fakeDictation) Recordings() ([]api.SavedDictation, error) {
	return d.recordings, nil
}

func (d *fakeDictation) Retry(id string) (string, error) {
	d.retried = append(d.retried, id)
	return d.retryText, d.retryErr
}

func (d *fakeDictation) DeleteRecording(id string) error {
	if d.deleteFailed {
		return errors.New("disk full")
	}
	d.deleted = append(d.deleted, id)
	return nil
}

// fakeIndicators records every indicator change in order.
type fakeIndicators struct {
	api.Indicators
	shown   []api.IndicatorInput
	hidden  []string
	changes []string
}

func (i *fakeIndicators) Show(input api.IndicatorInput) error {
	i.shown = append(i.shown, input)
	i.changes = append(i.changes, "show:"+input.Subtitle)
	return nil
}

func (i *fakeIndicators) Update(input api.IndicatorInput) error { return i.Show(input) }

func (i *fakeIndicators) Hide(id string) error {
	i.hidden = append(i.hidden, id)
	i.changes = append(i.changes, "hide")
	return nil
}

// last returns the most recent indicator shown.
func (i *fakeIndicators) last() api.IndicatorInput {
	if len(i.shown) == 0 {
		return api.IndicatorInput{}
	}
	return i.shown[len(i.shown)-1]
}

// fakeAI answers Ask with a reply, or fails, and records the prompts it was asked.
type fakeAI struct {
	api.AI
	available bool
	reply     string
	err       error
	prompts   []string
	options   []api.AIOptions
}

func (a *fakeAI) IsAvailable() (bool, error) { return a.available, nil }

func (a *fakeAI) Ask(prompt string, options api.AIOptions) (string, error) {
	a.prompts = append(a.prompts, prompt)
	a.options = append(a.options, options)
	return a.reply, a.err
}

func (a *fakeAI) Prepare(api.AIOptions) error { return nil }

// fakeOCR reads a fixed screen text.
type fakeOCR struct {
	api.OCR
	text string
	err  error
}

func (o fakeOCR) Screen(api.OCROptions) (api.OCRResult, error) {
	return api.OCRResult{Text: o.text}, o.err
}

type fakeApps struct {
	api.Apps
	front string
}

func (a fakeApps) Frontmost() (api.App, error) {
	if a.front == "" {
		return api.App{}, api.ErrNotAvailable
	}
	return api.App{ID: a.front}, nil
}

type fakeDesktop struct {
	api.Desktop
	front string
}

func (d fakeDesktop) Apps() api.Apps { return fakeApps{front: d.front} }

type fakeSystem struct {
	api.System
	capabilities map[string]bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.capabilities[name] }

type fakeLogs struct {
	api.Logs
	warnings []string
}

func (l *fakeLogs) Log(level api.LogLevel, message string, _ any) {
	if level == api.LogWarn {
		l.warnings = append(l.warnings, message)
	}
}

type fakeStorage struct {
	api.Storage
	values map[string][]byte
	failed bool
}

func (s *fakeStorage) Get(key string, into any) (bool, error) {
	raw, ok := s.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, into)
}

func (s *fakeStorage) Set(key string, value any) error {
	if s.failed {
		return errors.New("disk full")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[key] = raw
	return nil
}

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeNavigation struct{ api.Navigation }

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (fakeNavigation) Replace(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavReplace}
}

func (fakeNavigation) Pop() api.Result { return api.Result{Navigation: api.NavPop} }

// fakeContext is the context a test runs the built-in with. Its parts are pointers, so a test reads what the built-in did.
type fakeContext struct {
	api.Context
	dictation  *fakeDictation
	indicators *fakeIndicators
	ai         *fakeAI
	ocr        fakeOCR
	storage    *fakeStorage
	logs       *fakeLogs
	system     fakeSystem
	frontmost  string
}

func newContext() fakeContext {
	return fakeContext{
		dictation:  &fakeDictation{status: api.DictationIdle, available: true, devices: []api.DictationDevice{{ID: "mic-1", Title: "USB Mic", IsDefault: true}}},
		indicators: &fakeIndicators{},
		ai:         &fakeAI{available: true, reply: "Cleaned."},
		storage:    &fakeStorage{values: map[string][]byte{}},
		logs:       &fakeLogs{},
		system:     fakeSystem{capabilities: map[string]bool{"ocr": true}},
		frontmost:  "com.apple.Notes",
	}
}

func (c fakeContext) Std() context.Context       { return context.Background() }
func (c fakeContext) Storage() api.Storage       { return c.storage }
func (c fakeContext) Clock() api.Clock           { return fakeClock{now: testNow} }
func (c fakeContext) Dictation() api.Dictation   { return c.dictation }
func (c fakeContext) Indicators() api.Indicators { return c.indicators }
func (c fakeContext) AI() api.AI                 { return c.ai }
func (c fakeContext) OCR() api.OCR               { return c.ocr }
func (c fakeContext) System() api.System         { return c.system }
func (c fakeContext) Logs() api.Logs             { return c.logs }
func (c fakeContext) Desktop() api.Desktop       { return fakeDesktop{front: c.frontmost} }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }

// run runs one of the built-in's actions with the given args and form.
func run(t *testing.T, ext Extension, ctx api.Context, ref string, args map[string]string, form api.FormValues) api.Result {
	t.Helper()
	result, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}, Form: form})
	if err != nil {
		t.Fatalf("%s: %v", ref, err)
	}
	return result
}

// dictate runs the Dictate action, which starts a dictation when none runs and stops the running one.
func dictate(t *testing.T, ext Extension, ctx api.Context) api.Result {
	t.Helper()
	return run(t, ext, ctx, DictateCommandID, nil, nil)
}

// pasted returns the paste action of a result, or fails the test when the result pastes nothing.
func pasted(t *testing.T, result api.Result) api.Action {
	t.Helper()
	if result.Next == nil || result.Next.Kind != api.ActionPasteText {
		t.Fatalf("result %+v; want a paste", result)
	}
	return *result.Next
}

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
	ctx.storage.values[storageSettingsKey] = []byte(`{"cleanupWithAi":true,"useScreenContext":true,"screenTermLimit":10}`)
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
	if view == nil || view.Form == nil || len(view.Form.Fields) != 6 {
		t.Fatalf("settings view %+v; want the six 1.x fields", view)
	}
	form := api.FormValues{
		fieldDevice:      api.TextValue("mic-1"),
		fieldCleanup:     api.FlagValue(true),
		fieldDictionary:  api.TextValue("Kubernetes"),
		fieldClipboard:   api.FlagValue(true),
		fieldScreen:      api.FlagValue(true),
		fieldScreenLimit: api.TextValue("50"),
	}
	if result := run(t, ext, ctx, refSaveSettings, nil, form); result.Navigation != api.NavPop {
		t.Fatalf("navigation %q; want pop", result.Navigation)
	}
	settings := ext.readSettings(ctx)
	if settings.DeviceID != "mic-1" || settings.CleanupWithAI || settings.Dictionary != "Kubernetes" || !settings.CopyToClipboard || !settings.UseScreenContext || settings.ScreenTermLimit != 50 {
		t.Fatalf("settings %+v; want the saved choices, cleaning off without an account", settings)
	}
}

func TestLegacyChoicesAnswerUntilTheBuiltInSaves(t *testing.T) {
	ctx := newContext()
	legacy := Settings{DeviceID: "legacy-mic", ScreenTermLimit: 10}
	ext := New(Legacy{Settings: &legacy, History: []history.Entry{{ID: "old", Text: "kept", CreatedAt: 9}}})
	if got := ext.readSettings(ctx); got.DeviceID != "legacy-mic" || got.ScreenTermLimit != 10 {
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
func storedHistory(t *testing.T, ctx fakeContext) []history.Entry {
	t.Helper()
	var entries []history.Entry
	if _, err := ctx.storage.Get(storageHistoryKey, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func saveAll(t *testing.T, ctx fakeContext, entries ...history.Entry) {
	t.Helper()
	if err := writeHistory(ctx, entries); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
