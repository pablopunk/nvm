package submit

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

var testUpstream = Upstream{Owner: "pablopunk", Name: "nvm", OutputDir: "src/native/extensions"}

// fakeGH answers the CLI calls a submission makes, and records them.
type fakeGH struct {
	authed   bool
	user     string
	exists   bool
	calls    [][]string
	failNext string
}

func (g *fakeGH) run(args ...string) (string, error) {
	g.calls = append(g.calls, args)
	if g.failNext != "" && strings.Contains(strings.Join(args, " "), g.failNext) {
		return "", errors.New("boom")
	}
	joined := strings.Join(args, " ")
	switch {
	case args[0] == "--version":
		return "gh version 2", nil
	case args[0] == "auth":
		if !g.authed {
			return "", errors.New("not logged in")
		}
		return "", nil
	case args[0] == "api" && strings.Contains(joined, "api user"), slices.Contains(args, "user"):
		return g.user + "\n", nil
	case args[0] == "api" && strings.Contains(joined, "branches/main"):
		return "abc123\n", nil
	case args[0] == "api" && strings.Contains(joined, "contents/") && strings.Contains(joined, "?ref=main"):
		if g.exists {
			return "{}", nil
		}
		return "", errors.New("Not Found")
	case args[0] == "pr":
		return "https://github.com/pablopunk/nvm/pull/9\n", nil
	}
	return "", nil
}

func sampleExtension() Extension {
	return Extension{ID: "acme.notes", Title: "Notes", Files: []File{{Name: "extension.json", Contents: []byte("{}")}, {Name: "main.go", Contents: []byte("package main")}}}
}

// TestSubmitWithoutSignInAsksToSignIn checks the 1.x reason when the CLI is signed out: nothing is uploaded.
func TestSubmitWithoutSignInAsksToSignIn(t *testing.T) {
	g := &fakeGH{}
	_, err := Submit(g.run, testUpstream, sampleExtension())
	if !errors.Is(err, ErrNotSignedIn) || err.Error() != "Requires GitHub CLI (gh auth login)" {
		t.Fatalf("err = %v", err)
	}
	for _, c := range g.calls {
		if c[0] == "pr" || slices.Contains(c, "PUT") {
			t.Fatalf("a signed-out submission reached %v", c)
		}
	}
}

// TestSubmitForkedFlowOpensThePullRequest runs a user who is not the owner: the repository is forked, the branch is made
// on the fork, both files are uploaded to the output folder, and the pull request is opened from the fork.
func TestSubmitForkedFlowOpensThePullRequest(t *testing.T) {
	g := &fakeGH{authed: true, user: "pablo"}
	url, err := Submit(g.run, testUpstream, sampleExtension())
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/pablopunk/nvm/pull/9" {
		t.Fatalf("url = %q", url)
	}
	var forked, puts int
	var head string
	for _, c := range g.calls {
		joined := strings.Join(c, " ")
		if c[0] == "repo" && c[1] == "fork" {
			forked++
		}
		if slices.Contains(c, "PUT") {
			puts++
			if !strings.Contains(joined, "repos/pablo/nvm/contents/src/native/extensions/acme.notes/") {
				t.Fatalf("upload went to %q", joined)
			}
		}
		if c[0] == "pr" {
			head = c[slices.Index(c, "--head")+1]
		}
	}
	if forked != 1 || puts != 2 || head != "pablo:submit-extension-acme.notes" {
		t.Fatalf("forked %d, uploads %d, head %q", forked, puts, head)
	}
}

// TestSubmitByTheOwnerUsesTheRepositoryItself checks that the owner does not fork and opens the pull request from a branch.
func TestSubmitByTheOwnerUsesTheRepositoryItself(t *testing.T) {
	g := &fakeGH{authed: true, user: "pablopunk"}
	if _, err := Submit(g.run, testUpstream, sampleExtension()); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.calls {
		if c[0] == "repo" {
			t.Fatal("the owner forked the repository")
		}
		if c[0] == "pr" && c[slices.Index(c, "--head")+1] != "submit-extension-acme.notes" {
			t.Fatalf("head = %v", c)
		}
	}
}

// TestSubmitRefusesAnIDAlreadyInTheRepository checks the check 1.x makes before it writes anything.
func TestSubmitRefusesAnIDAlreadyInTheRepository(t *testing.T) {
	g := &fakeGH{authed: true, user: "pablopunk", exists: true}
	if _, err := Submit(g.run, testUpstream, sampleExtension()); !errors.Is(err, ErrAlreadyUpstream) {
		t.Fatalf("err = %v", err)
	}
}

// TestProbeReportsAMissingCLI checks that a missing CLI is neither installed nor signed in.
func TestProbeReportsAMissingCLI(t *testing.T) {
	missing := func(args ...string) (string, error) { return "", errors.New("not found") }
	if got := Probe(missing); got != (Status{}) {
		t.Fatalf("status = %+v", got)
	}
}
