package dictation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
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

func (a *fakeAI) Chats() api.AIChats { return nil }

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
	settings   *fakeDisplaySettings
}

// fakeDisplaySettings records the display text the built-in publishes.
type fakeDisplaySettings struct {
	api.Settings
	displays map[string]string
}

func (s *fakeDisplaySettings) PublishDisplay(id, text string) error {
	s.displays[id] = text
	return nil
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
		settings:   &fakeDisplaySettings{displays: map[string]string{}},
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
func (c fakeContext) Settings() api.Settings     { return c.settings }

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
