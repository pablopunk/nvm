package app

import "testing"

// TestOnlyTheAccountCommandThatFitsTheSessionIsListed checks the root commands: a device that is signed out offers Log in
// to Nevermind and no Log out.
func TestOnlyTheAccountCommandThatFitsTheSessionIsListed(t *testing.T) {
	p := goldenPalette(t)
	cmds := p.accountCommands()
	if len(cmds) != 1 || cmds[0].Title != "Log in to Nevermind" || cmds[0].Subtitle != "Connect this device to your Nevermind account" {
		t.Fatalf("signed out, the account commands are %+v; want Log in to Nevermind alone", cmds)
	}
}

// TestSignOutMessageNamesTheAccountAndAFailedRevoke checks the HUD message after a sign-out.
func TestSignOutMessageNamesTheAccountAndAFailedRevoke(t *testing.T) {
	if got := signOutMessage("ada@example.com", true); got != "Logged out of ada@example.com" {
		t.Fatalf("revoked sign-out says %q", got)
	}
	if got := signOutMessage("ada@example.com", false); got != "Logged out of ada@example.com (token revoke failed - check connection)" {
		t.Fatalf("failed revoke says %q", got)
	}
}
