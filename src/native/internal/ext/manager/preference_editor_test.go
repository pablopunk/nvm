package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakePreferences is the manager's side of the settings: two extensions with declared settings and the values saved.
type fakePreferences struct {
	mu     sync.Mutex
	values map[string]json.RawMessage
}

func newFakePreferences() *fakePreferences {
	return &fakePreferences{values: map[string]json.RawMessage{}}
}

func (f *fakePreferences) DeclaredPreferences() []ExtensionPreferences {
	return []ExtensionPreferences{
		{ID: "nevermind.settings", Title: "Settings", Settings: []api.SettingDefinition{
			{ID: "paletteShortcut", Title: "Open Nevermind Shortcut", Type: api.SettingString, Value: json.RawMessage(`""`)},
			{ID: "clipboardLimit", Title: "Clipboard History Size", Type: api.SettingString, Value: json.RawMessage(`"300"`),
				Choices: []api.SettingChoice{{Value: "300", Title: "300 items"}, {Value: "500", Title: "500 items"}, {Value: "1000", Title: "1000 items"}}},
		}},
		{ID: "acme.weather", Title: "Weather", Settings: []api.SettingDefinition{
			{ID: "metric", Title: "Metric Units", Description: "Celsius and kilometres", Type: api.SettingBoolean, Value: json.RawMessage(`true`)},
		}},
	}
}

func (f *fakePreferences) SettingValue(id, key string) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if value, ok := f.values[id+"/"+key]; ok {
		return value, nil
	}
	for _, extension := range f.DeclaredPreferences() {
		if definition, found := findDefinition(extension, key); found && extension.ID == id {
			return definition.Value, nil
		}
	}
	return nil, api.ErrNotFound
}

func (f *fakePreferences) SaveSettingForUser(id, key string, value json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[id+"/"+key] = value
	return nil
}

func (f *fakePreferences) stored(id, key string) string {
	value, _ := f.SettingValue(id, key)
	return string(value)
}

// uiQueue stands in for the UI thread: posted functions wait until the test runs them.
type uiQueue struct {
	mu     sync.Mutex
	posted []func()
	ready  chan struct{}
}

func newUIQueue() *uiQueue { return &uiQueue{ready: make(chan struct{}, 16)} }

func (q *uiQueue) post(fn func()) {
	q.mu.Lock()
	q.posted = append(q.posted, fn)
	q.mu.Unlock()
	q.ready <- struct{}{}
}

func (q *uiQueue) runNext(t *testing.T) {
	t.Helper()
	<-q.ready
	q.mu.Lock()
	fn := q.posted[0]
	q.posted = q.posted[1:]
	q.mu.Unlock()
	fn()
}

var recordedPalette = map[api.SettingRef]string{
	{Extension: "nevermind.settings", Setting: "paletteShortcut"}: "change it with Shortcuts().Set",
}

func runAtOnce(fn func()) { fn() }

func editorOver(source *fakePreferences, onUI func(func()), changed *[]string) PreferenceEditor {
	afterChange := func(id string) {
		if changed != nil {
			*changed = append(*changed, id)
		}
	}
	return NewPreferenceEditor(func() PreferenceSource { return source }, onUI, afterChange, recordedPalette)
}

func TestListingPreferencesMatchesEveryWordAndLeavesRecordedSettingsOut(t *testing.T) {
	editor := editorOver(newFakePreferences(), runAtOnce, nil)
	all, err := editor.List(context.Background(), "")
	if err != nil || len(all) != 2 {
		t.Fatalf("all = %+v, %v; want the two settings a value can set", all, err)
	}
	found, _ := editor.List(context.Background(), "weather KILOMETRES")
	if len(found) != 1 || found[0].ID != "metric" || found[0].ExtensionTitle != "Weather" || string(found[0].Value) != "true" {
		t.Fatalf("found = %+v", found)
	}
	if none, _ := editor.List(context.Background(), "clipboard weather"); len(none) != 0 {
		t.Fatalf("a record matched words from two settings: %+v", none)
	}
}

// Every refusal is invalid input with its reason, so nobody is asked to approve a change that cannot happen.
func TestPlanRefusesWhatCannotBeSaved(t *testing.T) {
	editor := editorOver(newFakePreferences(), runAtOnce, nil)
	cases := map[string]struct {
		extension, setting, value, reason string
	}{
		"unknown extension":     {"acme.missing", "metric", `true`, "no extension"},
		"unknown setting":       {"acme.weather", "wind", `true`, "has no setting"},
		"value of another type": {"acme.weather", "metric", `"yes"`, "expects a boolean"},
		"value not a choice":    {"nevermind.settings", "clipboardLimit", `"999"`, "accepts only"},
		"value it already has":  {"nevermind.settings", "clipboardLimit", `"300"`, "already has this value"},
		"recorded elsewhere":    {"nevermind.settings", "paletteShortcut", `"Command+K"`, "Shortcuts().Set"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := editor.Plan(context.Background(), c.extension, c.setting, json.RawMessage(c.value))
			if !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("err = %v; want invalid input naming %q", err, c.reason)
			}
		})
	}
}

func TestPlanNamesTheSettingItsValueAndTheRevision(t *testing.T) {
	editor := editorOver(newFakePreferences(), runAtOnce, nil)
	change, err := editor.Plan(context.Background(), "nevermind.settings", "clipboardLimit", json.RawMessage(`"500"`))
	if err != nil || change.Current.Title != "Clipboard History Size" || string(change.Current.Value) != `"300"` || change.Revision == "" {
		t.Fatalf("change = %+v, %v", change, err)
	}
	if again, _ := editor.Plan(context.Background(), "nevermind.settings", "clipboardLimit", json.RawMessage(`"500"`)); again.Revision != change.Revision {
		t.Fatal("the revision of an unchanged value moved")
	}
}

// A save waits for the UI thread and runs there, followed by the app's redraw of the extension.
func TestSetSavesOnTheUIThreadAndRedraws(t *testing.T) {
	source, queue := newFakePreferences(), newUIQueue()
	var changed []string
	editor := editorOver(source, queue.post, &changed)
	done := make(chan error, 1)
	go func() {
		_, err := editor.Set(context.Background(), "acme.weather", "metric", json.RawMessage(`false`), "")
		done <- err
	}()
	if got := source.stored("acme.weather", "metric"); got != "true" {
		t.Fatalf("metric = %s before the UI thread ran the save", got)
	}
	queue.runNext(t)
	if err := <-done; err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := source.stored("acme.weather", "metric"); got != "false" || fmt.Sprint(changed) != "[acme.weather]" {
		t.Fatalf("metric = %s, redrawn %v", got, changed)
	}
}

// A caller that gives up before the UI thread takes the save leaves the setting as it was, also when the thread runs
// later.
func TestSetCancelledBeforeItStartsSavesNothing(t *testing.T) {
	source, queue := newFakePreferences(), newUIQueue()
	editor := editorOver(source, queue.post, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := editor.Set(ctx, "acme.weather", "metric", json.RawMessage(`false`), "")
	if err == nil {
		t.Fatal("a cancelled save reported success")
	}
	queue.runNext(t)
	if got := source.stored("acme.weather", "metric"); got != "true" {
		t.Fatalf("metric = %s after a cancelled save", got)
	}
}

// A change approved against an older value does not overwrite the value the user chose while the question waited.
func TestSetRefusesAStaleRevision(t *testing.T) {
	source := newFakePreferences()
	editor := editorOver(source, runAtOnce, nil)
	change, err := editor.Plan(context.Background(), "nevermind.settings", "clipboardLimit", json.RawMessage(`"500"`))
	if err != nil {
		t.Fatal(err)
	}
	_ = source.SaveSettingForUser("nevermind.settings", "clipboardLimit", json.RawMessage(`"1000"`))
	_, err = editor.Set(context.Background(), "nevermind.settings", "clipboardLimit", json.RawMessage(`"500"`), change.Revision)
	if !errors.Is(err, errPreferenceMoved) || !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v; want the stale change refused", err)
	}
	if got := source.stored("nevermind.settings", "clipboardLimit"); got != `"1000"` {
		t.Fatalf("clipboardLimit = %s; the user's newer choice was overwritten", got)
	}
}

func TestPreferencesAreUnavailableBeforeTheExtensionsStart(t *testing.T) {
	editor := NewPreferenceEditor(func() PreferenceSource { return nil }, runAtOnce, nil, nil)
	if _, err := editor.List(context.Background(), ""); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishedDisplaySettingsAreNotPreferences(t *testing.T) {
	manifest := api.Manifest{Settings: []api.SettingDefinition{{ID: "device"}, {ID: "engine"}}, Publishes: []string{"device"}}
	settings := userChosenSettings(manifest)
	if len(settings) != 1 || settings[0].ID != "engine" {
		t.Fatalf("settings = %+v", settings)
	}
}
