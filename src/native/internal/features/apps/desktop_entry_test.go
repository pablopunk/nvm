package apps

import (
	"testing"
)

// Ported from os.linux.test.ts: "parses visible Linux desktop entries without filesystem access",
// "matches Linux app commands and window classes against injected process names", and the Linux desktop
// entry scan root contract is covered by the uninstall discovery tests.

// Ported: "parses visible Linux desktop entries without filesystem access".
func TestParseDesktopEntryReadsNameCommandAndWMClass(t *testing.T) {
	entry, ok := ParseDesktopEntry(`
[Desktop Entry]
Name=Example App
Exec=env FOO=bar /opt/example/bin/example --open %U %c
StartupWMClass=ExampleApp
`)
	if !ok {
		t.Fatal("visible entry refused")
	}
	want := DesktopEntry{Name: "Example App", Command: "env FOO=bar /opt/example/bin/example --open", WMClass: "ExampleApp"}
	if entry != want {
		t.Fatalf("entry = %+v, want %+v", entry, want)
	}
	for _, body := range []string{
		"[Desktop Entry]\nNoDisplay=true\nName=Hidden\nExec=hidden",
		"[Desktop Entry]\nHidden=true\nName=Hidden\nExec=hidden",
		"[Desktop Entry]\nExec=missing-name",
		"[Desktop Entry]\nName=Missing Exec",
	} {
		if _, ok := ParseDesktopEntry(body); ok {
			t.Errorf("entry %q accepted", body)
		}
	}
}

// Ported: "matches Linux app commands and window classes against injected process names".
func TestDesktopEntryMatchesProcessNames(t *testing.T) {
	names := func(list ...string) map[string]bool {
		out := map[string]bool{}
		for _, name := range list {
			out[lowerASCII(name)] = true
		}
		return out
	}
	cases := []struct {
		entry DesktopEntry
		names map[string]bool
		want  bool
	}{
		{DesktopEntry{Command: "env DEBUG=1 /opt/Example/bin/example", WMClass: "ExampleApp"}, names("other-process", "EXAMPLE"), true},
		{DesktopEntry{Command: "example", WMClass: "ExampleApp"}, names("exampleapp"), true},
		{DesktopEntry{Command: "example", WMClass: "ExampleApp"}, names("unrelated"), false},
	}
	for _, c := range cases {
		if got := c.entry.MatchesProcessNames(c.names); got != c.want {
			t.Errorf("MatchesProcessNames(%+v, %v) = %v, want %v", c.entry, c.names, got, c.want)
		}
	}
}

// Hostile input beyond 1.x: a program name longer than the kernel's 15-byte comm matches its truncated form, and
// quoted or path-qualified commands resolve to the program's base name.
func TestDesktopEntryMatchesTruncatedAndQuotedCommands(t *testing.T) {
	if got := ExecutableName(`"/opt/My App/bin/my-long-application-name" --flag`); got != "my-long-application-name" {
		t.Fatalf("quoted ExecutableName = %q", got)
	}
	long := DesktopEntry{Command: "/usr/bin/my-long-application-name"}
	if !long.MatchesProcessNames(map[string]bool{"my-long-applica": true}) {
		t.Fatal("truncated comm did not match the long program name")
	}
	if long.MatchesProcessNames(map[string]bool{"my-long-applica-other": true}) {
		t.Fatal("a different process matched")
	}
}

// Hostile input: an Exec line made only of environment assignments has no program and matches nothing.
func TestDesktopEntryWithOnlyAssignmentsMatchesNothing(t *testing.T) {
	entry, ok := ParseDesktopEntry("[Desktop Entry]\nName=Odd\nExec=FOO=1 BAR=2 %u\n")
	if !ok {
		t.Fatal("entry refused")
	}
	if name := ExecutableName(entry.Command); name != "" {
		t.Fatalf("executable = %q, want none", name)
	}
	if entry.MatchesProcessNames(map[string]bool{"": true}) {
		t.Fatal("empty process name matched")
	}
}
