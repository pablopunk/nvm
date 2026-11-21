package app

import (
	"path"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// legacyAccountDocument is the account.json an earlier build wrote for one signed-in account on the production origin.
const legacyAccountDocument = `{"schemaVersion":1,"data":{"environment":"production","baseUrl":"","connections":{"https://api.nvm.fyi":{"userId":"user_1","email":"pat@example.test","role":"member","connectedAt":"2026-11-04T10:00:00Z"}}}}`

// signedOutAccountDocument is an account.json of this build with nobody signed in.
const signedOutAccountDocument = `{"schemaVersion":1,"data":{"environment":"production","baseUrl":"","connections":{}}}`

// accountTokenOnProduction is the bearer the earlier build stored in the keychain for the production origin.
const accountTokenOnProduction = "nvm-legacy-canary-token"

// legacyAccountFolder points the earlier build's folder at a temporary one for the test and restores it afterwards.
func legacyAccountFolder(t *testing.T, folder string) {
	t.Helper()
	previous := legacyConfigDir
	legacyConfigDir = func() string { return folder }
	t.Cleanup(func() { legacyConfigDir = previous })
}

// relaunch quits the app that ran over set's stores, as the quit listener does, and starts a new one over them.
func relaunch(t *testing.T, previous *App, set *fake.Set, dir string) *App {
	t.Helper()
	if previous != nil && previous.userStore != nil {
		if err := previous.userStore.Close(); err != nil {
			t.Fatal(err)
		}
	}
	next := goldenPalette(t)
	next.attachServices(set.Ports(), "darwin", dir, time.UTC)
	return next
}

// TestSignInFromTheEarlierBuildSurvivesTheFolderMoveAndARestart copies the earlier account document and checks that
// the sign-in is there at launch and again after a restart, with the token still in the keychain under its old name.
func TestSignInFromTheEarlierBuildSurvivesTheFolderMoveAndARestart(t *testing.T) {
	const legacy = "/nvm-test/nvm-native"
	legacyAccountFolder(t, legacy)
	set := fake.NewSet()
	writeTestFile(t, set, legacy+"/account.json", legacyAccountDocument)
	origin := productionOrigin(t)
	if err := auth.NewVault(set.Keychain).SaveToken(origin, backend.Token(accountTokenOnProduction)); err != nil {
		t.Fatal(err)
	}
	const dir = "/nvm-test/user-data"
	if err := set.FS.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	first := relaunch(t, nil, set, dir)
	assertSignedInAs(t, first, "pat@example.test", "the first launch after the move")
	second := relaunch(t, first, set, dir)
	assertSignedInAs(t, second, "pat@example.test", "the launch after a restart")
	if _, err := set.FS.ReadFile(legacy + "/account.json"); err != nil {
		t.Fatalf("the earlier account document was removed: %v", err)
	}
}

// TestEarlierAccountDocumentDoesNotOverwriteTheCurrentOne keeps a sign-out: once this build has its own account
// document, the earlier one is never copied again, even when this build holds no account.
func TestEarlierAccountDocumentDoesNotOverwriteTheCurrentOne(t *testing.T) {
	const legacy = "/nvm-test/nvm-native"
	legacyAccountFolder(t, legacy)
	set := fake.NewSet()
	writeTestFile(t, set, legacy+"/account.json", legacyAccountDocument)
	const dir = "/nvm-test/user-data"
	if err := set.FS.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, set, dir+"/account.json", signedOutAccountDocument)

	app := relaunch(t, nil, set, dir)
	if app.accountSource().Connected {
		t.Fatal("the earlier account was copied over the current document after a sign-out")
	}
}

// productionOrigin is the origin the production environment resolves to.
func productionOrigin(t *testing.T) string {
	t.Helper()
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatal(err)
	}
	return env.BaseURL
}

// assertSignedInAs fails unless app shows the account as connected with email.
func assertSignedInAs(t *testing.T, app *App, email, when string) {
	t.Helper()
	snapshot := app.accountSource()
	if !snapshot.Connected || snapshot.Email != email {
		t.Fatalf("%s: connected = %v, email = %q; want the signed-in account %q", when, snapshot.Connected, snapshot.Email, email)
	}
}

// writeTestFile writes data to name on set's fake disk, creating its folder first.
func writeTestFile(t *testing.T, set *fake.Set, name, data string) {
	t.Helper()
	if err := set.FS.MkdirAll(path.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := set.FS.WriteFile(name, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
