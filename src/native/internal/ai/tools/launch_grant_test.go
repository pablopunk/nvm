package tools

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// waives reports whether waiver lets one call of the named tool, with arguments, run without asking.
func (r *rig) waives(waiver agent.Waiver, name string, arguments any) bool {
	r.t.Helper()
	tool, ok := r.set.Tools.Lookup(name)
	if !ok {
		r.t.Fatalf("no tool %s", name)
	}
	data, _ := json.Marshal(arguments)
	call := backend.ToolCall{ID: "c", Type: "function", Function: backend.ToolFunction{Name: name, Arguments: string(data)}}
	return waiver.Waive(call, tool)
}

func openURL(address string) map[string]string { return map[string]string{"url": address} }
func openApp(name string) map[string]string    { return map[string]string{"name": name} }

// A plain request in the current message lets exactly its launch run once without a second question.
func TestAPlainRequestGrantsItsOneLaunchOnce(t *testing.T) {
	r := newRig(t)
	cases := map[string]struct {
		message, tool string
		args          map[string]string
	}{
		"bare host":        {"open github.com", "open_url", openURL("https://github.com/")},
		"host spelling":    {"Open github.com.", "open_url", openURL("https://GitHub.com:443")},
		"full address":     {"go to https://example.com/a?b=1", "open_url", openURL("https://example.com/a?b=1")},
		"exact app":        {"open Safari", "open_app", openApp("safari")},
		"app with suffix":  {"please launch Safari.app", "open_app", openApp("Safari")},
		"politely, at end": {"open Calculator please", "open_app", openApp("Calculator")},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			grant := r.set.LaunchGrantFor(c.message)
			if !r.waives(grant, c.tool, c.args) {
				t.Fatalf("%q did not grant %s %v", c.message, c.tool, c.args)
			}
			if r.waives(grant, c.tool, c.args) {
				t.Fatal("the grant let a second call through")
			}
		})
	}
}

// Only a message that does nothing but ask to open one thing grants it; anything else asks as before.
func TestAMessageThatIsNotAPlainRequestGrantsNothing(t *testing.T) {
	r := newRig(t)
	for _, message := range []string{
		"don't open github.com",
		"do not open Safari",
		"never open github.com",
		"open github.com, not gitlab.com",
		"open github.com and paste my notes there",
		"can you open github.com?",
		"I opened github.com yesterday",
		"thanks",
		"",
	} {
		grant := r.set.LaunchGrantFor(message)
		if r.waives(grant, "open_url", openURL("https://github.com/")) || r.waives(grant, "open_app", openApp("Safari")) {
			t.Errorf("%q granted a launch", message)
		}
	}
}

// The grant covers the normalized address, not one like it: any changed component asks.
func TestAChangedAddressComponentIsNotGranted(t *testing.T) {
	r := newRig(t)
	message := "open https://github.com/pablopunk/nvm?tab=readme"
	for name, address := range map[string]string{
		"scheme":    "http://github.com/pablopunk/nvm?tab=readme",
		"host":      "https://gist.github.com/pablopunk/nvm?tab=readme",
		"port":      "https://github.com:8443/pablopunk/nvm?tab=readme",
		"path":      "https://github.com/pablopunk/nvm/secret?tab=readme",
		"query":     "https://github.com/pablopunk/nvm?tab=readme&token=abc",
		"no query":  "https://github.com/pablopunk/nvm",
		"fragment":  "https://github.com/pablopunk/nvm?tab=readme#notes",
		"user name": "https://me@github.com/pablopunk/nvm?tab=readme",
	} {
		if r.waives(r.set.LaunchGrantFor(message), "open_url", openURL(address)) {
			t.Errorf("a changed %s was granted: %s", name, address)
		}
	}
	if !r.waives(r.set.LaunchGrantFor(message), "open_url", openURL("HTTPS://GitHub.com/pablopunk/nvm?tab=readme")) {
		t.Fatal("the same address in other letter case was not granted")
	}
}

// An app name that is only part of an installed name, or that several apps share, grants nothing.
func TestAnAmbiguousAppNameGrantsNothing(t *testing.T) {
	r := newRig(t)
	for _, call := range []string{"Calendar", "Calculator", "cal"} {
		if r.waives(r.set.LaunchGrantFor("open cal"), "open_app", openApp(call)) {
			t.Errorf("a partial name granted %s", call)
		}
	}
	r.shell.SetApps([]platform.App{
		{Name: "Code", ID: "a", Path: "/Applications/Code.app"},
		{Name: "Code", ID: "b", Path: "/Users/u/Applications/Code.app"},
	}, nil)
	if r.waives(r.set.LaunchGrantFor("open Code"), "open_app", openApp("Code")) {
		t.Fatal("a name two apps share was granted")
	}
}

// A grant covers only its own target and tool: another app, a link for an app, and a clipboard write all ask.
func TestAGrantCoversOnlyItsOwnTarget(t *testing.T) {
	r := newRig(t)
	if r.waives(r.set.LaunchGrantFor("open Safari"), "open_app", openApp("Calculator")) {
		t.Fatal("a grant for Safari opened Calculator")
	}
	if r.waives(r.set.LaunchGrantFor("open Safari"), "open_url", openURL("https://safari.com/")) {
		t.Fatal("an app grant opened a link")
	}
	if r.waives(r.set.LaunchGrantFor("open github.com"), "open_app", openApp("github.com")) {
		t.Fatal("a link grant opened an app")
	}
	if r.waives(r.set.LaunchGrantFor("open github.com"), "copy_text", map[string]string{"text": "github.com"}) {
		t.Fatal("a launch grant wrote the clipboard")
	}
}
