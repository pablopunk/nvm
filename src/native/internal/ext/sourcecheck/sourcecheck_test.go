package sourcecheck

import (
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// source wraps the body of a main.go that imports the api, so each test states only the calls it checks.
func source(body string) map[string][]byte {
	return map[string][]byte{"main.go": []byte("package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\n" + body + "\n")}
}

// TestAppleScriptWithoutSystemIsFlagged is the owner's refusal: a theme toggle that runs AppleScript declares no system
// capability, so the check names the missing one before anything loads.
func TestAppleScriptWithoutSystemIsFlagged(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"System Events\"", api.ShellOptions{})
	return api.Result{}, err
}`)
	manifest := api.Manifest{Identity: api.Identity{ID: "user.system-theme", Title: "System Theme"}}
	missing := UndeclaredUses(manifest, files)
	if len(missing) != 1 || missing[0].Capability != api.CapabilitySystem {
		t.Fatalf("missing = %+v; want one system use", missing)
	}
	if missing[0].Call != "ctx.Desktop().Shell().AppleScript()" {
		t.Fatalf("call = %q; want the full chain the source wrote", missing[0].Call)
	}
	if message := Explain(missing); !strings.Contains(message, `"system"`) || !strings.Contains(message, "capabilities") {
		t.Fatalf("explanation %q does not tell the model what to declare", message)
	}
}

// TestDeclaredCapabilityPassesTheCheck: the same source is accepted once its manifest declares system.
func TestDeclaredCapabilityPassesTheCheck(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"System Events\"", api.ShellOptions{})
	return api.Result{}, err
}`)
	manifest := api.Manifest{Identity: api.Identity{ID: "user.system-theme", Title: "System Theme"}, Capabilities: []api.Capability{api.CapabilitySystem}}
	if missing := UndeclaredUses(manifest, files); len(missing) != 0 {
		t.Fatalf("missing = %+v; want none once system is declared", missing)
	}
}

// TestOpenExternalNeedsDesktopFilesNotSystem: the host gates OpenExternal on desktop.files, even though it is a Shell
// method, so the more specific rule must win over the Shell rule.
func TestOpenExternalNeedsDesktopFilesNotSystem(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	return api.Result{}, ctx.Desktop().Shell().OpenExternal("https://example.com")
}`)
	uses := UsesOf(files)
	if len(uses) != 1 || uses[0].Capability != api.CapabilityDesktopFiles {
		t.Fatalf("uses = %+v; want one desktop.files use", uses)
	}
}

// TestNestedReceiversAreOneCall: the outer call of a chain is the one listed, once per capability, and the inner
// receivers are not listed again as calls of their own.
func TestNestedReceiversAreOneCall(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	ctx.Desktop().Shell().Exec("ls", nil, api.ShellOptions{})
	ctx.Desktop().Shell().Which("ls")
	return api.Result{}, nil
}`)
	uses := UsesOf(files)
	if len(uses) != 1 || uses[0].Capability != api.CapabilitySystem {
		t.Fatalf("uses = %+v; want one system use for the two shell calls", uses)
	}
}

// TestUsesOfFindsEachServiceByItsRule covers the gated services the host refuses without a declaration.
func TestUsesOfFindsEachServiceByItsRule(t *testing.T) {
	cases := map[string]api.Capability{
		`ctx.Network().Fetch(api.FetchRequest{URL: "https://example.com"})`: api.CapabilityNetwork,
		`ctx.Secrets().Set("token", "x")`:                                   api.CapabilitySecrets,
		`ctx.Settings().Set("theme", "dark")`:                               api.CapabilitySettingsWrite,
		`ctx.Clipboard().List(api.ClipboardQuery{})`:                        api.CapabilityClipboardHistory,
		`ctx.Desktop().Clipboard().ReadText()`:                              api.CapabilityDesktopClipboard,
		`ctx.Desktop().Apps().List()`:                                       api.CapabilityDesktopApps,
		`ctx.Desktop().Menus().Snapshot()`:                                  api.CapabilityDesktopMenus,
		`ctx.Desktop().Selection().Files()`:                                 api.CapabilityDesktopFiles,
		`ctx.Desktop().Selection().Text()`:                                  api.CapabilityDesktopSelection,
		`ctx.Desktop().Selection().ReplaceText("x")`:                        api.CapabilityDesktopSelection,
		`ctx.Desktop().Files().Open("/tmp/x")`:                              api.CapabilityDesktopFiles,
		`ctx.Desktop().WindowManagement().Windows(api.WindowQuery{})`:       api.CapabilityWindowManagement,
		`ctx.OCR().Screen(api.OCROptions{})`:                                api.CapabilityOCR,
		`ctx.AI().Ask("hi", api.AIOptions{})`:                               api.CapabilityAI,
		`ctx.Shortcuts().List()`:                                            api.CapabilityShortcuts,
	}
	for call, want := range cases {
		files := source("func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {\n\t_ = " + call + "\n\treturn api.Result{}, nil\n}")
		uses := UsesOf(files)
		if len(uses) != 1 || uses[0].Capability != want {
			t.Errorf("%s: uses = %+v; want %s", call, uses, want)
		}
	}
}

// TestUngatedServicesAreNotFlagged: storage, logs and the system clock need no capability, so they never block a reply.
func TestUngatedServicesAreNotFlagged(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_ = ctx.Storage().Set("k", "v")
	_ = ctx.Logs().Info("hello")
	_ = ctx.Clock().Now()
	return api.Result{}, nil
}`)
	if uses := UsesOf(files); len(uses) != 0 {
		t.Fatalf("uses = %+v; want none", uses)
	}
}

// TestTestFilesAndManifestAreNotSource: a *_test.go file or the manifest does not count as the extension's calls.
func TestTestFilesAndManifestAreNotSource(t *testing.T) {
	files := map[string][]byte{
		"extension.json": []byte(`{"title":"Shell"}`),
		"main_test.go":   []byte("package main\nfunc f(ctx api.Context) { ctx.Desktop().Shell().Exec(\"ls\", nil, api.ShellOptions{}) }"),
	}
	if uses := UsesOf(files); len(uses) != 0 {
		t.Fatalf("uses = %+v; want none", uses)
	}
}

// TestDisplayTitlesReadsCommandsAndItemsDirectAndElided reads the titles a source declares: a command in a slice
// literal, a root item from a helper, and a duplicate title counted once.
func TestDisplayTitlesReadsCommandsAndItemsDirectAndElided(t *testing.T) {
	files := source(`func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "toggle-theme", Title: "Toggle System Theme"}}, nil
}

func toggleItem() api.Item { return api.Item{ID: "toggle", Title: "toggle system   theme"} }

func RootItems(ctx api.Context) ([]api.Item, error) { return []api.Item{toggleItem(), {Title: "Clock"}}, nil }`)
	titles := DisplayTitles(files)
	want := []string{"Clock", "Toggle System Theme"}
	if !slices.Equal(titles, want) {
		t.Fatalf("titles = %q; want %q", titles, want)
	}
	if !SameTitle("Toggle System Theme", " toggle  system theme ") {
		t.Fatal("titles that differ only in case and spacing must be the same title")
	}
}

// TestRulesAreTheHostsCapabilities: every rule names a capability the api knows, so a rule cannot drift to a name the
// manifest would reject as unknown.
func TestRulesAreTheHostsCapabilities(t *testing.T) {
	known := api.KnownCapabilities()
	for _, rule := range Rules() {
		if !slices.Contains(known, rule.Capability) {
			t.Errorf("rule %v names %q, which the api does not know", rule.Path, rule.Capability)
		}
	}
}
