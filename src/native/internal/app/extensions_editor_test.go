package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// editorUnderTest is the extensions port over a fake controller: Clock is on and shows a menu bar item, Weather is off,
// Broken does not load, and Dictation is a built-in with a menu bar item. Reviews and menu bar syncs are recorded.
type editorUnderTest struct {
	editor   extensionsEditor
	ctl      *fakeExtensions
	reviews  []string
	synced   int
	uiQueued int
}

func newEditorUnderTest() *editorUnderTest {
	u := &editorUnderTest{ctl: &fakeExtensions{listings: []manager.Listing{
		{ID: "acme.clock", Title: "Clock", Enabled: true, Capabilities: []api.Capability{api.CapabilityNetwork}},
		{ID: "acme.weather", Title: "Weather"},
		{ID: "acme.broken", Title: "Broken", Problems: []string{"does not compile"}},
	}}}
	u.editor = extensionsEditor{
		controller: func() extensionController { return u.ctl },
		builtIn:    func(id string) bool { return id == "nevermind.dictation" },
		menuBars: func() []manager.MenuBarExtension {
			return []manager.MenuBarExtension{{ID: "acme.clock", Title: "Clock"}, {ID: "nevermind.dictation", Title: "Dictation"}}
		},
		visibility:     &menuBarVisibility{},
		onUI:           func(fn func()) { u.uiQueued++; fn() },
		openReview:     func(_ extensionController, l manager.Listing) { u.reviews = append(u.reviews, l.ID) },
		menuBarChanged: func() { u.synced++ },
	}
	return u
}

func TestTheExtensionsPortListsFolderExtensionsAndMenuBarBuiltIns(t *testing.T) {
	u := newEditorUnderTest()
	u.editor.visibility.setShown("nevermind.dictation", false)
	records := u.editor.List()
	want := []api.ExtensionRecord{
		{ID: "acme.clock", Title: "Clock", Enabled: true, MenuBar: true, MenuBarShown: true, Capabilities: []api.Capability{api.CapabilityNetwork}},
		{ID: "acme.weather", Title: "Weather"},
		{ID: "acme.broken", Title: "Broken"},
		{ID: "nevermind.dictation", Title: "Dictation", Enabled: true, BuiltIn: true, MenuBar: true},
	}
	if len(records) != len(want) {
		t.Fatalf("records = %+v", records)
	}
	for i := range want {
		if records[i].ID != want[i].ID || records[i].Enabled != want[i].Enabled || records[i].BuiltIn != want[i].BuiltIn ||
			records[i].MenuBar != want[i].MenuBar || records[i].MenuBarShown != want[i].MenuBarShown || !slices.Equal(records[i].Capabilities, want[i].Capabilities) {
			t.Fatalf("record %d = %+v; want %+v", i, records[i], want[i])
		}
	}
}

// OpenReview opens the review on the UI thread and never calls Enable: only the review's own Enable row turns it on.
func TestOpenReviewOpensTheReviewAndEnablesNothing(t *testing.T) {
	u := newEditorUnderTest()
	if err := u.editor.OpenReview("acme.weather"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(u.reviews, []string{"acme.weather"}) || u.uiQueued != 1 {
		t.Fatalf("reviews = %v, UI hops %d", u.reviews, u.uiQueued)
	}
	if len(u.ctl.calls) != 0 {
		t.Fatalf("opening a review changed the extension: %v", u.ctl.calls)
	}
}

func TestThePortRefusesWhatItCannotChange(t *testing.T) {
	cases := map[string]func(extensionsEditor) error{
		"review an enabled extension": func(e extensionsEditor) error { return e.OpenReview("acme.clock") },
		"review a broken extension":   func(e extensionsEditor) error { return e.OpenReview("acme.broken") },
		"review a built-in":           func(e extensionsEditor) error { return e.OpenReview("nevermind.dictation") },
		"disable a built-in":          func(e extensionsEditor) error { return e.Disable("nevermind.dictation") },
		"disable an unknown id":       func(e extensionsEditor) error { return e.Disable("nope") },
		"disable a disabled one":      func(e extensionsEditor) error { return e.Disable("acme.weather") },
		"menu bar of an extension without one": func(e extensionsEditor) error {
			return e.SetMenuBarItem("acme.weather", false)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			u := newEditorUnderTest()
			if err := change(u.editor); !errors.Is(err, api.ErrInvalidInput) {
				t.Fatalf("err = %v; want a refusal", err)
			}
			if len(u.ctl.calls) != 0 || len(u.reviews) != 0 || u.synced != 0 {
				t.Fatalf("a refusal changed something: calls %v, reviews %v, syncs %d", u.ctl.calls, u.reviews, u.synced)
			}
		})
	}
}

func TestDisableAndMenuBarChangesReachTheHost(t *testing.T) {
	u := newEditorUnderTest()
	if err := u.editor.Disable("acme.clock"); err != nil {
		t.Fatal(err)
	}
	if err := u.editor.SetMenuBarItem("nevermind.dictation", false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(u.ctl.calls, []string{"disable acme.clock"}) || u.editor.visibility.shown("nevermind.dictation") || u.synced != 1 {
		t.Fatalf("calls %v, dictation shown %v, syncs %d", u.ctl.calls, u.editor.visibility.shown("nevermind.dictation"), u.synced)
	}
}
