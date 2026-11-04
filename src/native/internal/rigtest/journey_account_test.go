//go:build rig

package rigtest

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// The account journeys sign in the way a user does: the root row "Settings" opens Settings, its "Account" row opens
// Nevermind Account, the first row of that page starts the device flow, and the fake backend approves the code in
// place of the browser's verification page.

const (
	settingsRootRow    = "Settings"
	accountSettingsRow = "Account"
	startSignInRow     = "Log in to Nevermind"
	signInPollWait     = 15 * time.Second // the app polls the fake once a second; a sign-in ends within a few polls
	signedInTemplate   = "Logged in as "
)

// openAccountPage summons the palette, opens Settings from the root and then the Account row in it.
func openAccountPage(t *testing.T, app *App) {
	t.Helper()
	must(t, app.SummonPalette())
	must(t, app.Type("settings"))
	waitForRow(t, app, settingsRootRow, elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor("Search Settings", elementWait)
	must(t, err)
	must(t, app.Type("account"))
	waitForRow(t, app, accountSettingsRow, elementWait)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Nevermind Account", elementWait)
	must(t, err)
}

// signInThroughAccount runs the device flow from the Account page and returns the code. It fails the test unless the
// account shows the fixture account as signed in.
func signInThroughAccount(t *testing.T, app *App, fake *FakeBackend) string {
	t.Helper()
	openAccountPage(t, app)
	_, err := app.WaitFor(startSignInRow, elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	code, err := fake.WaitForDeviceCode(elementWait)
	must(t, err)
	_, err = app.WaitFor(code, elementWait)
	must(t, err)
	must(t, fake.ApproveDeviceCode(code))
	_, err = app.WaitFor(signedInTemplate+fixtures.FixtureAccount.Email, signInPollWait)
	must(t, err)
	return code
}
