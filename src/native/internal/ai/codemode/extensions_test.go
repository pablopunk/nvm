package codemode

import (
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeExtensions is the host's extensions: Clock is on with a shown menu bar item, Weather is off, and Dictation is a
// built-in with a menu bar item. It records what it was asked to do. It has no way to enable an extension, as the port.
type fakeExtensions struct {
	mu       sync.Mutex
	records  []api.ExtensionRecord
	calls    []string
	reviewed []string
}

func newFakeExtensions() *fakeExtensions {
	return &fakeExtensions{records: []api.ExtensionRecord{
		{ID: "acme.clock", Title: "Clock", Enabled: true, MenuBar: true, MenuBarShown: true},
		{ID: "acme.weather", Title: "Weather"},
		{ID: "nevermind.dictation", Title: "Dictation", Enabled: true, BuiltIn: true, MenuBar: true, MenuBarShown: true},
	}}
}

func (f *fakeExtensions) List() []api.ExtensionRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]api.ExtensionRecord(nil), f.records...)
}

func (f *fakeExtensions) Disable(id string) error {
	return f.change("disable "+id, id, func(r *api.ExtensionRecord) { r.Enabled = false })
}

func (f *fakeExtensions) OpenReview(id string) error {
	f.mu.Lock()
	f.reviewed = append(f.reviewed, id)
	f.mu.Unlock()
	return f.change("review "+id, id, func(*api.ExtensionRecord) {})
}

func (f *fakeExtensions) SetMenuBarItem(id string, shown bool) error {
	return f.change("menubar "+id, id, func(r *api.ExtensionRecord) { r.MenuBarShown = shown })
}

func (f *fakeExtensions) change(call, id string, apply func(*api.ExtensionRecord)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	for i := range f.records {
		if f.records[i].ID == id {
			apply(&f.records[i])
		}
	}
	return nil
}

func (f *fakeExtensions) record(id string) api.ExtensionRecord {
	for _, record := range f.List() {
		if record.ID == id {
			return record
		}
	}
	return api.ExtensionRecord{}
}

func (f *fakeExtensions) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func extensionsFixture(t *testing.T) (*fixture, *fakeExtensions) {
	extensions := newFakeExtensions()
	return newFixture(t, func(opts *Options) { opts.Extensions = extensions }), extensions
}

func TestExtensionOperationsAreClassifiedByEffect(t *testing.T) {
	want := map[string]Class{
		protocol.OpList: ClassRead, protocol.OpDisable: ClassSetting, protocol.OpOpenReview: ClassWrite, protocol.OpSetMenuBarItem: ClassSetting,
	}
	rules := extensionRules()
	if len(rules) != len(want) {
		t.Fatalf("the extensions table has %d rules; this test pins %d", len(rules), len(want))
	}
	for op, class := range want {
		rule, ok := rules[key{protocol.ServiceExtensions, op}]
		if !ok || rule.class != class {
			t.Errorf("extensions.%s = %+v (listed %v); want %s", op, rule.class, ok, class)
		}
		if rule.label == "" || rule.describe == nil || rule.journal == nil {
			t.Errorf("extensions.%s lacks a label, a description or a journal line", op)
		}
	}
}

// Turning an extension off asks with its title from the host, and runs once allowed.
func TestTurningAnExtensionOffAsksWithItsTitleAndRunsWhenAllowed(t *testing.T) {
	f, extensions := extensionsFixture(t)
	f.steps.decide = allow
	result := f.run(`return nil, ctx.Extensions().Disable("acme.clock")`)
	if result.Failed || extensions.record("acme.clock").Enabled {
		t.Fatalf("result = %+v, clock %+v", result, extensions.record("acme.clock"))
	}
	row := lastRow(t, f, "Turning off an extension")
	t.Logf("question: %s\nmodel reads: %s", row.detail, result.Text)
	if !row.asked || row.detail != "Turn off the extension “Clock”" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "turned off the extension “Clock” (the user allowed it)")
}

func TestADeclinedExtensionChangeChangesNothing(t *testing.T) {
	f, extensions := extensionsFixture(t)
	f.steps.decide = decline
	result := f.run(`return nil, ctx.Extensions().SetMenuBarItem("acme.clock", false)`)
	if !result.Failed || !extensions.record("acme.clock").MenuBarShown || extensions.callCount() != 0 {
		t.Fatalf("result = %+v, calls %v", result, extensions.calls)
	}
	t.Logf("model reads: %s", result.Text)
	contains(t, result.Text, "hide the menu bar item of “Clock”: the user declined")
}

// An unknown id, a built-in, or a change to the state an extension already has fails without asking.
func TestAnExtensionChangeTheHostRefusesFailsWithoutAsking(t *testing.T) {
	cases := map[string]struct{ body, reason string }{
		"unknown":           {`return nil, ctx.Extensions().Disable("nope")`, "no extension has the id"},
		"built-in disable":  {`return nil, ctx.Extensions().Disable("nevermind.dictation")`, "built in and always on"},
		"built-in review":   {`return nil, ctx.Extensions().OpenReview("nevermind.dictation")`, "built in and always on"},
		"already off":       {`return nil, ctx.Extensions().Disable("acme.weather")`, "is already off"},
		"review enabled":    {`return nil, ctx.Extensions().OpenReview("acme.clock")`, "is already on"},
		"no menu bar":       {`return nil, ctx.Extensions().SetMenuBarItem("acme.weather", true)`, "has no menu bar item"},
		"menu bar as it is": {`return nil, ctx.Extensions().SetMenuBarItem("acme.clock", true)`, "is already shown"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, extensions := extensionsFixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v", result)
			}
			for _, row := range f.steps.snapshot() {
				if row.asked {
					t.Fatalf("a refused extension change was put to the user: %+v", row)
				}
			}
			if extensions.callCount() != 0 {
				t.Fatalf("a refused change reached the host: %v", extensions.calls)
			}
		})
	}
}

// Opening a review only shows the review screen: it does not ask, and the extension stays off until the user enables it
// there. The model is told so.
func TestOpeningAReviewNeverEnablesTheExtension(t *testing.T) {
	f, extensions := extensionsFixture(t)
	result := f.run(`return nil, ctx.Extensions().OpenReview("acme.weather")`)
	if result.Failed || len(extensions.reviewed) != 1 || extensions.reviewed[0] != "acme.weather" {
		t.Fatalf("result = %+v, reviewed %v", result, extensions.reviewed)
	}
	if extensions.record("acme.weather").Enabled {
		t.Fatal("opening the review enabled the extension")
	}
	row := lastRow(t, f, "Opening an extension review")
	t.Logf("row: %s\nmodel reads: %s", row.detail, result.Text)
	if row.asked {
		t.Fatalf("opening a review asked: %+v", row)
	}
	contains(t, result.Text, "opened the review of the extension “Weather” in the palette; it stays off until the user enables it there")
}

func TestListingExtensionsDoesNotAsk(t *testing.T) {
	f, _ := extensionsFixture(t)
	result := f.run(`return len(ctx.Extensions().List()), nil`)
	if result.Failed || !strings.Contains(result.Text, "Result: 3") {
		t.Fatalf("result = %+v", result)
	}
}
