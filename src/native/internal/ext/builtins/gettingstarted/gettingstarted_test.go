package gettingstarted

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeStorage holds the done flag and nothing else.
type fakeStorage struct {
	api.Storage
	done   bool
	stored bool
	failed bool
}

func (s *fakeStorage) Get(key string, into any) (bool, error) {
	if key != storageKey || !s.stored {
		return false, nil
	}
	*into.(*bool) = s.done
	return true, nil
}

func (s *fakeStorage) Set(key string, value any) error {
	if s.failed {
		return errors.New("disk full")
	}
	s.done, s.stored = value.(bool), true
	return nil
}

// fakeViews counts the root refreshes the built-in asks for.
type fakeViews struct {
	api.Views
	invalidations int
}

func (v *fakeViews) InvalidateRoot() { v.invalidations++ }

// fakeShortcuts reports one palette shortcut with the label the host spells for it.
type fakeShortcuts struct {
	api.Shortcuts
	label string
}

func (s fakeShortcuts) Palette() api.ShortcutRecord {
	return api.ShortcutRecord{Accelerator: "Command+Alt+K", Label: s.label}
}

// fakeAccount reports the signed-in state the test sets.
type fakeAccount struct {
	api.Account
	signedIn bool
}

func (a fakeAccount) SignedIn() bool { return a.signedIn }

type fakeNavigation struct {
	api.Navigation
}

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (fakeNavigation) Pop() api.Result { return api.Result{Navigation: api.NavPop} }

// fakeContext serves the services the built-in uses; the others are absent.
type fakeContext struct {
	api.Context
	storage   *fakeStorage
	views     *fakeViews
	shortcuts fakeShortcuts
	account   fakeAccount
}

func (c fakeContext) Storage() api.Storage       { return c.storage }
func (c fakeContext) Views() api.Views           { return c.views }
func (c fakeContext) Shortcuts() api.Shortcuts   { return c.shortcuts }
func (c fakeContext) Account() api.Account       { return c.account }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }

func newContext(label string) fakeContext {
	return fakeContext{storage: &fakeStorage{}, views: &fakeViews{}, shortcuts: fakeShortcuts{label: label}}
}

func rootEntry(t *testing.T, ext Extension, ctx api.Context) api.Item {
	t.Helper()
	items, err := ext.RootItems(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("RootItems = %+v, err %v; want the one Welcome row", items, err)
	}
	return items[0]
}

func TestWelcomeIsTheRootRowUntilDismissed(t *testing.T) {
	item := rootEntry(t, New(false), newContext(""))
	if item.Title != "Welcome to Nevermind" || item.Score != welcomeScore || item.Section != "Commands" || item.LearnKey != "guide" {
		t.Fatalf("row = %+v, want Welcome at 90 in Commands", item)
	}
	if item.Primary == nil || item.Primary.Title != "Get Started" || len(item.Actions) != 1 || item.Actions[0].Title != "Dismiss welcome" {
		t.Fatalf("actions = %+v / %+v, want Get Started and Dismiss welcome", item.Primary, item.Actions)
	}
}

func TestDismissStoresTheFlagAndRunsTheRootAgain(t *testing.T) {
	ctx := newContext("")
	if _, err := New(false).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: dismissActionID}}); err != nil {
		t.Fatal(err)
	}
	if !ctx.storage.done || ctx.views.invalidations != 1 {
		t.Fatalf("done = %v, invalidations = %d; want the flag stored and one root refresh", ctx.storage.done, ctx.views.invalidations)
	}
	item := rootEntry(t, New(false), ctx)
	if item.Title != "Help" || item.Score != helpScore || item.Primary.Title != "Open Help" || len(item.Actions) != 0 {
		t.Fatalf("row after dismissal = %+v, want Help at 14 with no dismiss action", item)
	}
}

func TestLegacyDoneFlagAnswersUntilStored(t *testing.T) {
	if item := rootEntry(t, New(true), newContext("")); item.Title != "Help" {
		t.Fatalf("title = %q, want Help from the legacy flag", item.Title)
	}
}

func TestDismissFailureIsReturned(t *testing.T) {
	ctx := newContext("")
	ctx.storage.failed = true
	if _, err := New(false).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: dismissActionID}}); err == nil {
		t.Fatal("dismiss succeeded although storage refused the flag")
	}
	if ctx.views.invalidations != 0 {
		t.Fatal("the root was refreshed for a dismissal that was not stored")
	}
}

func TestWelcomeIsFoundByItsWordsAtAliasScore(t *testing.T) {
	ext := New(false)
	for _, query := range []string{"welcome", "getting", "how to", "help"} {
		items, err := ext.SearchItems(newContext(""), query)
		if err != nil || len(items) != 1 || items[0].Score != 45 {
			t.Fatalf("query %q: items = %+v, err %v; want the row at 45", query, items, err)
		}
	}
	for _, query := range []string{"", "x", "calculator"} {
		if items, _ := ext.SearchItems(newContext(""), query); len(items) != 0 {
			t.Fatalf("query %q: items = %+v, want none", query, items)
		}
	}
}

func TestGuideOpensWithTheLiveShortcutLabel(t *testing.T) {
	ctx := newContext("⌘⌥K")
	result, err := New(false).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: openActionID}})
	if err != nil || result.View == nil || result.Navigation != api.NavPush {
		t.Fatalf("open = %+v, err %v; want the guide pushed", result, err)
	}
	row := itemWithTitle(t, result.View, "Keyboard shortcut set")
	if row.Subtitle != "⌘⌥K opens Nevermind from anywhere. You can change it any time." {
		t.Fatalf("subtitle = %q", row.Subtitle)
	}
	if row.Primary == nil || row.Primary.Kind != api.ActionRecordPaletteShortcut {
		t.Fatalf("primary = %+v, want the record-palette-shortcut action", row.Primary)
	}
}

func TestGuideWithoutAShortcutOffersToSetOne(t *testing.T) {
	rows := guideRows("", false)
	for _, row := range rows {
		if row.ID == "getting-started-shortcut" && (row.Title != "Open Nevermind from anywhere" || row.action != actionSetShortcut) {
			t.Fatalf("unset shortcut row = %+v", row)
		}
	}
}

func TestGuideEndsWithDoneForNow(t *testing.T) {
	rows := guideRows("", false)
	last := rows[len(rows)-1]
	if last.ID != "getting-started-done" || last.action != actionFinish {
		t.Fatalf("last guide row = %+v, want Done for now", last)
	}
}

func TestDoneForNowStoresTheFlagAndPops(t *testing.T) {
	ctx := newContext("")
	result, err := New(false).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: finishActionID}})
	if err != nil || result.Navigation != api.NavPop || !ctx.storage.done || ctx.views.invalidations != 1 {
		t.Fatalf("finish = %+v, err %v, done %v, invalidations %d", result, err, ctx.storage.done, ctx.views.invalidations)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	if _, err := New(false).RunAction(newContext(""), api.Invocation{Ref: api.ActionRef{ID: "nope"}}); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
}

func itemWithTitle(t *testing.T, view *api.View, title string) api.Item {
	t.Helper()
	for _, item := range view.Items {
		if item.Title == title {
			return item
		}
	}
	t.Fatalf("no guide row titled %q", title)
	return api.Item{}
}

// guideTitles lists the titles of the guide's rows, in order.
func guideTitles(t *testing.T, ctx api.Context) []string {
	t.Helper()
	var titles []string
	for _, item := range guideView(ctx).Items {
		titles = append(titles, item.Title)
	}
	return titles
}

func TestGuideOffersSignInWhileSignedOutAfterTheShortcutRow(t *testing.T) {
	ctx := newContext("")
	want := []string{"Find a command", "Run it", "Open Nevermind from anywhere", "Sign in to Nevermind", "Permissions when needed", "Done for now"}
	if got := strings.Join(guideTitles(t, ctx), "|"); got != strings.Join(want, "|") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	view := guideView(ctx)
	row := itemWithTitle(t, &view, "Sign in to Nevermind")
	if row.Subtitle != "Optional: connect your account to use AI chats." || row.Disabled {
		t.Fatalf("sign-in row = %+v", row)
	}
	if row.Primary == nil || row.Primary.Kind != api.ActionAccount || row.Primary.Op != api.AccountSignIn || row.Primary.Title != "Sign in" {
		t.Fatalf("primary = %+v, want the account sign-in action", row.Primary)
	}
}

func TestGuideShowsTheAccountAsDisabledWhenSignedIn(t *testing.T) {
	ctx := newContext("")
	ctx.account.signedIn = true
	view := guideView(ctx)
	row := itemWithTitle(t, &view, "Nevermind account")
	if row.Subtitle != "You are already signed in. Account access is only needed for AI chats." || !row.Disabled || row.Primary != nil {
		t.Fatalf("account row = %+v, want a disabled information row", row)
	}
}
