package sourcecheck

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestAppleScriptMakesAnExtensionMacOSOnly: a source that calls AppleScript works on macOS only, with no manifest field.
func TestAppleScriptMakesAnExtensionMacOSOnly(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"Calendar\" to activate", api.ShellOptions{})
	return api.Result{}, err
}`)
	supported := SupportedPlatforms(files)
	if !supported.Includes(api.OSDarwin) || supported.Includes(api.OSLinux) || supported.Includes(api.OSWindows) {
		t.Fatalf("supported = %v; want macOS only", []string(supported))
	}
	if supported.Label() != "macOS only" {
		t.Fatalf("label = %q; want %q", supported.Label(), "macOS only")
	}
	uses := PlatformUsesOf(files)
	if len(uses) != 1 || uses[0].Call != "ctx.Desktop().Shell().AppleScript()" {
		t.Fatalf("uses = %+v; want the one AppleScript call", uses)
	}
}

// TestASourceWithoutOSSpecificCallsWorksEverywhere: ordinary services narrow nothing.
func TestASourceWithoutOSSpecificCallsWorksEverywhere(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().Exec("ls", nil, api.ShellOptions{})
	return api.Result{}, err
}`)
	if supported := SupportedPlatforms(files); !supported.Everywhere() || supported.Label() != "" {
		t.Fatalf("supported = %v; want every OS", []string(supported))
	}
	if note := PlatformNote(files); note != "" {
		t.Fatalf("note = %q; want none", note)
	}
}

// TestEveryOSSpecificOperationIsFoundInASource: each entry of the table is detected by the chain it names.
func TestEveryOSSpecificOperationIsFoundInASource(t *testing.T) {
	for _, operation := range api.PlatformOperations {
		call := "ctx." + strings.Join(operation.Call, "().") + "(nil)"
		files := source("func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {\n\t" + call + "\n\treturn api.Result{}, nil\n}")
		uses := PlatformUsesOf(files)
		if len(uses) != 1 || uses[0].Operation.Name() != operation.Name() {
			t.Errorf("%s: uses = %+v; want the operation found", operation.Name(), uses)
		}
	}
}

// TestSupportedPlatformsIsTheIntersection: operations that share no OS leave no OS, and the nil set narrows nothing.
func TestSupportedPlatformsIsTheIntersection(t *testing.T) {
	if got := api.OnlyOn(api.OSDarwin, api.OSLinux).Intersect(api.OnlyOn(api.OSDarwin)); got.Label() != "macOS only" {
		t.Fatalf("macOS and Linux with macOS = %q; want macOS only", got.Label())
	}
	if got := api.OnlyOn(api.OSDarwin).Intersect(api.OnlyOn(api.OSLinux)); got.Everywhere() || got.Includes(api.OSDarwin) || got.Includes(api.OSLinux) {
		t.Fatalf("macOS with Linux = %v; want no OS", []string(got))
	}
	if got := api.Platforms(nil).Intersect(api.OnlyOn(api.OSLinux)); got.Label() != "Linux only" {
		t.Fatalf("every OS with Linux = %q; want Linux only", got.Label())
	}
}

// TestASourceThatAsksTheOSDecidesForItself: an extension that branches on System().OS has another way elsewhere, so the
// host does not limit it.
func TestASourceThatAsksTheOSDecidesForItself(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if ctx.System().OS() == "darwin" {
		_, _ = ctx.Desktop().Shell().AppleScript("beep", api.ShellOptions{})
	}
	return api.Result{}, nil
}`)
	if supported := SupportedPlatforms(files); !supported.Everywhere() {
		t.Fatalf("supported = %v; want every OS for a source that branches on the OS", []string(supported))
	}
}

// TestPlatformNoteIsInformation: the note says the OS and the call, and never calls the source wrong.
func TestPlatformNoteIsInformation(t *testing.T) {
	files := source(`func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("beep", api.ShellOptions{})
	return api.Result{}, err
}`)
	note := PlatformNote(files)
	for _, want := range []string{"macOS only", "AppleScript", "installed and enabled"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q lacks %q", note, want)
		}
	}
}
