package app

import "testing"

// TestSecondInstanceLinkIsTheSignInURLOnTheCommandLine checks the link a second launch carries: the nvm:// argument is
// taken, and the program's other arguments are not.
func TestSecondInstanceLinkIsTheSignInURLOnTheCommandLine(t *testing.T) {
	link, ok := secondInstanceLink([]string{"nevermind", "--flag", "nvm://auth/callback?code=abc", "other"})
	if !ok || link != "nvm://auth/callback?code=abc" {
		t.Fatalf("link = %q, %v; want the nvm:// argument", link, ok)
	}
	if link, ok := secondInstanceLink([]string{"nevermind", "--flag"}); ok {
		t.Fatalf("link = %q from a launch without one", link)
	}
}
