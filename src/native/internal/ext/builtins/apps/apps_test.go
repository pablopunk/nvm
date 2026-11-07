package apps

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

var fixtureApps = []api.App{
	{Name: "Zed", ID: "dev.zed", Path: "/Applications/Zed.app", FirstSeenMs: 1},
	{Name: "Beta", ID: "com.example.beta", Path: "/Applications/Beta.app", FirstSeenMs: 3},
	{Name: "Alpha", ID: "com.example.alpha", Path: "/Applications/Alpha.app", FirstSeenMs: 3},
}

func titlesOf(items []api.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}

// TestRootListsEveryAppNewestFirstThenByName pins the empty root's order to 1.x: first seen, newest first, then by name.
func TestRootListsEveryAppNewestFirstThenByName(t *testing.T) {
	ctx := newTestContext(&fakeApps{list: fixtureApps})
	items, err := New().RootItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alpha", "Beta", "Zed"}; !slices.Equal(titlesOf(items), want) {
		t.Fatalf("root = %v, want %v", titlesOf(items), want)
	}
	for _, item := range items {
		if item.Score != scoreRootApplication || item.Section != search.KindApplications || item.Subtitle != "" {
			t.Fatalf("row %q = score %v section %q subtitle %q", item.Title, item.Score, item.Section, item.Subtitle)
		}
	}
}

// TestRowCarriesItsKeysAndIcon: the row's learning key, its shortcut key, its drag file and its system icon.
func TestRowCarriesItsKeysAndIcon(t *testing.T) {
	ctx := newTestContext(&fakeApps{list: fixtureApps[:1]})
	items, _ := New().RootItems(ctx)
	item := items[0]
	if item.LearnKey != "app:dev.zed" || item.CustomizeAs != "app:/Applications/Zed.app" {
		t.Fatalf("keys = %q / %q", item.LearnKey, item.CustomizeAs)
	}
	if item.File == nil || item.File.Path != "/Applications/Zed.app" {
		t.Fatalf("file = %+v, want the app bundle to drag", item.File)
	}
	if item.Primary == nil || item.Primary.Title != "Open Zed" || !item.Primary.DismissAfterRun {
		t.Fatalf("primary = %+v, want Open Zed dismissing the palette", item.Primary)
	}
}

// TestRunningAppsAreMarkedAndUninstallIsOfferedWhenSupported: the Running subtitle, and Uninstall in the menu.
func TestRunningAppsAreMarkedAndUninstallIsOfferedWhenSupported(t *testing.T) {
	ctx := newTestContext(&fakeApps{list: fixtureApps, running: map[string]bool{"/Applications/Beta.app": true}})
	items, _ := New().RootItems(ctx)
	for _, item := range items {
		if item.Title == "Beta" && item.Subtitle != "Running" {
			t.Fatalf("Beta subtitle = %q, want Running", item.Subtitle)
		}
		if len(item.Actions) != 1 || item.Actions[0].Title != "Uninstall "+item.Title+"…" {
			t.Fatalf("%q actions = %+v, want Uninstall", item.Title, item.Actions)
		}
	}
	ctx.system.capabilities = map[string]bool{}
	items, _ = New().RootItems(ctx)
	if len(items[0].Actions) != 0 {
		t.Fatalf("actions without uninstall support = %+v, want none", items[0].Actions)
	}
}

// TestSearchMatchesNamesAndAliasesAndKeepsTheRest: a name finds the app, an alias finds it when the name does not.
func TestSearchMatchesNamesAndAliasesAndKeepsTheRest(t *testing.T) {
	ctx := newTestContext(&fakeApps{list: fixtureApps})
	ctx.shortcuts.aliases = map[string][]string{"app:/Applications/Zed.app": {"editor"}}
	items, _ := New().SearchItems(ctx, "zed")
	if want := []string{"Zed"}; !slices.Equal(titlesOf(items), want) {
		t.Fatalf("search zed = %v, want %v", titlesOf(items), want)
	}
	items, _ = New().SearchItems(ctx, "editor")
	if want := []string{"Zed"}; !slices.Equal(titlesOf(items), want) {
		t.Fatalf("search editor = %v, want the aliased app %v", titlesOf(items), want)
	}
	if items, _ := New().SearchItems(ctx, "nothing"); len(items) != 0 {
		t.Fatalf("search nothing = %v, want none", titlesOf(items))
	}
}

// TestOpenLaunchesTheAppByItsPath: Enter on a row launches the app it names.
func TestOpenLaunchesTheAppByItsPath(t *testing.T) {
	apps := &fakeApps{list: fixtureApps}
	ctx := newTestContext(apps)
	items, _ := New().RootItems(ctx)
	if _, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refOpen, Args: map[string]string{argPath: items[0].Primary.Ref.Args[argPath]}}}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(apps.launched, []string{items[0].Primary.Ref.Args[argPath]}) {
		t.Fatalf("launched = %v", apps.launched)
	}
}

// TestCommandsAreTheTwoQuitCommandsWithTheirNativeKeys pins the commands to the native titles, scores and keys.
func TestCommandsAreTheTwoQuitCommandsWithTheirNativeKeys(t *testing.T) {
	commands, _ := New().Commands(nil)
	if len(commands) != 2 {
		t.Fatalf("commands = %+v", commands)
	}
	force, quitAll := commands[0], commands[1]
	if force.ID != "force-quit-apps" || force.Title != "Force Quit Apps" || force.Score != 14 || !force.Customizable {
		t.Fatalf("force quit = %+v", force)
	}
	if quitAll.ID != "quit-all-apps" || quitAll.Title != "Quit All Apps" || quitAll.Score != 12 || !quitAll.Customizable {
		t.Fatalf("quit all = %+v", quitAll)
	}
}

// TestForceQuitListsTheRunningAppsAndAsksBeforeEachOne: the list shows only running apps, and each force quit asks.
func TestForceQuitListsTheRunningAppsAndAsksBeforeEachOne(t *testing.T) {
	apps := &fakeApps{list: fixtureApps, running: map[string]bool{"/Applications/Zed.app": true}}
	ctx := newTestContext(apps)
	result, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: ForceQuitCommandID}})
	if err != nil || result.View == nil || result.View.Title != "Force Quit Apps" {
		t.Fatalf("result = %+v, %v; want the Force Quit Apps list", result, err)
	}
	if want := []string{"Zed"}; !slices.Equal(titlesOf(result.View.Items), want) {
		t.Fatalf("list = %v, want %v", titlesOf(result.View.Items), want)
	}
	quit := result.View.Items[0].Primary
	if quit == nil || quit.Confirm == nil || quit.Confirm.Title != "Force Quit?" || quit.Confirm.ConfirmLabel != "Force Quit" {
		t.Fatalf("force quit action = %+v, want the Force Quit confirmation", quit)
	}
	if quit.Confirm.Message != "Force quit Zed? Unsaved data may be lost." {
		t.Fatalf("confirmation message = %q", quit.Confirm.Message)
	}
}

// TestForceQuitReportsWhatHappened: the toast names the app and the outcome, as the native quit did.
func TestForceQuitReportsWhatHappened(t *testing.T) {
	cases := map[string]struct {
		outcome api.QuitResult
		want    string
	}{
		"not running": {api.QuitResult{}, "Zed is not running."},
		"forced":      {api.QuitResult{WasRunning: true, Quit: true}, "Force quit Zed."},
		"refused":     {api.QuitResult{WasRunning: true}, "Could not quit Zed."},
	}
	for name, tc := range cases {
		apps := &fakeApps{forceQuit: tc.outcome}
		result, err := New().RunAction(newTestContext(apps), api.Invocation{Ref: api.ActionRef{ID: refForceQuit, Args: map[string]string{argPath: "/Applications/Zed.app", argName: "Zed"}}})
		if err != nil || result.Toast == nil || result.Toast.Message != tc.want {
			t.Fatalf("%s: toast = %+v, %v; want %q", name, result.Toast, err, tc.want)
		}
	}
}

// TestQuitAllAsksOnceThenQuitsTheRunningApps: the command asks, and only the confirmed run asks the host to quit.
func TestQuitAllAsksOnceThenQuitsTheRunningApps(t *testing.T) {
	apps := &fakeApps{list: fixtureApps}
	ctx := newTestContext(apps)
	result, _ := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: QuitAllCommandID}})
	if result.Next == nil || result.Next.Confirm == nil || result.Next.Confirm.Title != "Quit All Apps?" {
		t.Fatalf("result = %+v, want a Quit All Apps confirmation", result)
	}
	if apps.quitAsked != 0 {
		t.Fatal("Quit All Apps quit before it was confirmed")
	}
	if _, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refQuitAll}}); err != nil || apps.quitAsked != 1 {
		t.Fatalf("quit all asked %d times, err %v", apps.quitAsked, err)
	}
}
