//go:build rig

package rigtest

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// J12 (PLAN.md 3.1): sign in through the device flow. The fake backend issues the code, the Account page shows it
// while the sign-in is pending, and the page shows the account once the fake approves the code. The token is never
// written to the app's log. Sign-out and the offline repeat are not covered yet.
func TestJourneyJ12SignInThroughTheDeviceFlow(t *testing.T) {
	fake := StartFakeBackend(t)
	app := Launch(t, Options{Backend: fake.URL})

	openAccountPage(t, app)
	_, err := app.WaitFor(startSignInRow, elementWait)
	must(t, err)
	shoot(t, app, "j12-1-signed-out")

	must(t, app.Key("Return"))
	code, err := fake.WaitForDeviceCode(elementWait)
	must(t, err)
	_, err = app.WaitFor(code, elementWait)
	must(t, err)
	shoot(t, app, "j12-2-pending")

	must(t, fake.ApproveDeviceCode(code))
	_, err = app.WaitFor(signedInTemplate+fixtures.FixtureAccount.Email, signInPollWait)
	must(t, err)
	shoot(t, app, "j12-3-signed-in")

	for _, path := range []string{fixtures.DeviceInitiatePath, fixtures.DeviceExchangePath} {
		if !requestedPath(fake, path) {
			t.Errorf("the app never called %s on the fake backend", path)
		}
	}
	if strings.Contains(app.Log(), fixtures.FixtureAccount.Token) {
		t.Fatal("the app log holds the sign-in token")
	}
	hidePalette(t, app)
}

// requestedPath reports whether the fake backend received a request for path.
func requestedPath(fake *FakeBackend, path string) bool {
	for _, request := range fake.Requests() {
		if request.Path == path {
			return true
		}
	}
	return false
}
