package guard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const home = "/home/u"

// tree is a canonicalizer over an in-memory file tree. A path under a symbolic link resolves through the link's target,
// as the file system does; a path under one of the existing prefixes resolves to itself; anything else does not exist.
func tree(links map[string]string, existing ...string) func(string) (string, error) {
	return func(path string) (string, error) {
		for link, target := range links {
			if path == link || strings.HasPrefix(path, link+"/") {
				return target + path[len(link):], nil
			}
		}
		for _, prefix := range existing {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return path, nil
			}
		}
		return "", os.ErrNotExist
	}
}

// newGuard returns the guard the tools use: the home folder is the only root.
func newGuard(links map[string]string) *Guard {
	return New(Config{Home: home, Roots: []string{home}}, tree(links, home, "/etc", "/usr"))
}

// TestCheckTable is the path policy table: what the model may name and what it may not.
func TestCheckTable(t *testing.T) {
	g := newGuard(nil)
	tests := []struct {
		name  string
		raw   string
		allow bool
	}{
		{"plain file", "~/docs/notes.txt", true},
		{"absolute file", "/home/u/docs/notes.txt", true},
		{"at-sign spelling", "@~/docs/notes.txt", true},
		{"file URL", "file:///home/u/docs/notes.txt", true},
		{"no-break space in the spelling", "~/ docs/notes.txt", true},
		{"ssh keys", "~/.ssh/id_ed25519", false},
		{"ssh keys by absolute path", "/home/u/.ssh/id_ed25519", false},
		{"ssh keys through a dot path", "~/docs/../.ssh/config", false},
		{"ssh keys by file URL", "file:///home/u/.ssh/id", false},
		{"ssh keys in another case", "/HOME/U/.SSH/id", false},
		{"aws credentials", "~/.aws/credentials", false},
		{"git credentials", "~/.git-credentials", false},
		{"keychain", "~/Library/Keychains/login.keychain-db", false},
		{"browser profile", "~/Library/Application Support/Google/Chrome/Default/Cookies", false},
		{"dotenv with secrets", "~/project/.env", false},
		{"dotenv variant", "~/project/.env.production", false},
		{"dotenv template stays readable", "~/project/.env.example", true},
		{"outside home", "../../etc/passwd", false},
		{"absolute outside home", "/etc/hosts", false},
		{"home itself", "~", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := g.Check(tt.raw, home)
			if decision.Allow != tt.allow {
				t.Fatalf("Check(%q) = %+v, want allow=%v", tt.raw, decision, tt.allow)
			}
			if !tt.allow && decision.Reason == "" {
				t.Fatal("a refusal carries no reason")
			}
		})
	}
}

// TestSymbolicLinksAreJudgedWhereTheyPoint checks that a link into a credential location, or out of the roots, is
// refused even though its own name is harmless.
func TestSymbolicLinksAreJudgedWhereTheyPoint(t *testing.T) {
	g := newGuard(map[string]string{
		"/home/u/key":    "/home/u/.ssh/id_ed25519",
		"/home/u/out":    "/etc",
		"/home/u/notes":  "/home/u/docs/notes.txt",
		"/home/u/nested": "/home/u/.aws",
	})
	cases := map[string]bool{
		"~/key":          false,
		"~/out/hosts":    false,
		"~/notes":        true,
		"~/nested/creds": false,
	}
	for raw, allow := range cases {
		if got := g.Check(raw, home).Allow; got != allow {
			t.Fatalf("Check(%q) allowed = %v, want %v", raw, got, allow)
		}
	}
}

// TestRelativePathsResolveFromTheGivenFolder checks cwd handling: a relative path is joined to cwd first.
func TestRelativePathsResolveFromTheGivenFolder(t *testing.T) {
	g := newGuard(nil)
	if got := g.Resolve("notes.txt", "/home/u/docs"); got != filepath.Join("/home/u/docs", "notes.txt") {
		t.Fatalf("Resolve = %q", got)
	}
	if g.Check("../.ssh/id", "/home/u/docs").Allow {
		t.Fatal("a relative path into .ssh was allowed")
	}
}

// TestCredentialsOutsideRootsStillFail checks that a guard with extra denied locations refuses them, and that an
// allowed location carves out only extra denials.
func TestCredentialsOutsideRootsStillFail(t *testing.T) {
	g := New(Config{Home: home, ExtraDenied: []string{"/srv/secrets"}, Allowed: []string{"/srv/secrets/public"}},
		tree(nil, home, "/srv"))
	if g.Check("/srv/secrets/token", home).Allow {
		t.Fatal("an extra denied location was allowed")
	}
	if !g.Check("/srv/secrets/public/readme", home).Allow {
		t.Fatal("an allowed carve-out was refused")
	}
	if g.Check("~/.ssh/id", home).Allow {
		t.Fatal("an allowed carve-out reached into a home credential location")
	}
}

// TestUnresolvableRootsStillGuard checks that a missing root does not weaken the guard.
func TestUnresolvableRootsStillGuard(t *testing.T) {
	failing := func(string) (string, error) { return "", errors.New("no such file") }
	g := New(Config{Home: home, Roots: []string{home}}, failing)
	if g.Check("~/.ssh/id", home).Allow {
		t.Fatal("a guard with no resolver allowed a credential path")
	}
}
