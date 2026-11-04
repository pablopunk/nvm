package app

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/submit"
)

// withGitHubStatus sets the cached CLI state the menu reads, so no process runs in the test.
func withGitHubStatus(p *App, status submit.Status) {
	p.github = &githubProbe{status: status, at: time.Now(), run: func(args ...string) (string, error) {
		return "", errors.New("the CLI is not called in this test")
	}}
}

// TestSubmitAsPRIsHiddenWithoutTheCLI checks that a missing GitHub CLI removes Submit as PR, as 1.x does.
func TestSubmitAsPRIsHiddenWithoutTheCLI(t *testing.T) {
	p := goldenPalette(t)
	withGitHubStatus(p, submit.Status{})
	if _, ok := p.submitAction("acme.notes"); ok {
		t.Fatal("Submit as PR is shown without the GitHub CLI")
	}
}

// TestSubmitAsPRIsDisabledUntilSignedIn checks the 1.x reason shown while the CLI is installed but not signed in.
func TestSubmitAsPRIsDisabledUntilSignedIn(t *testing.T) {
	p := goldenPalette(t)
	withGitHubStatus(p, submit.Status{Installed: true})
	action, ok := p.submitAction("acme.notes")
	if !ok || action.Disabled != "Requires GitHub CLI (gh auth login)" {
		t.Fatalf("Submit as PR = %+v, %v; want disabled with the sign-in reason", action, ok)
	}
}

// TestSubmitAsPRRunsWhenSignedIn checks that a signed-in CLI leaves the action enabled.
func TestSubmitAsPRRunsWhenSignedIn(t *testing.T) {
	p := goldenPalette(t)
	withGitHubStatus(p, submit.Status{Installed: true, Authed: true})
	action, ok := p.submitAction("acme.notes")
	if !ok || action.Disabled != "" || action.Run == nil {
		t.Fatalf("Submit as PR = %+v, %v; want an enabled action", action, ok)
	}
}
