//go:build automation

package app

import (
	"log"
	"os"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// rigBackendEnv names the origin of the rig's fake backend. The rig sets it on every launch (internal/rigtest).
const rigBackendEnv = "NVM_RIG_BACKEND"

// rigUnreachableOrigin is the origin an automation launch uses when no fake backend was named. The discard port
// refuses every connection, so a rig launch never reaches the production backend.
const rigUnreachableOrigin = "http://127.0.0.1:9"

// applyRigBackend points the account at the rig's fake backend and pins it there, so Settings and deep links cannot
// move it to production. The origin must be a loopback HTTP origin, or the
// app stops rather than fall back to production. Sign-in opens no browser, since the rig approves the code through
// the fake, and tokens stay in memory, so no test writes to the keyring of the machine it runs on.
func applyRigBackend(deps *auth.Deps) {
	origin := strings.TrimSpace(os.Getenv(rigBackendEnv))
	if origin == "" {
		origin = rigUnreachableOrigin
	}
	environment, err := backend.NewEnvironment(backend.EnvironmentDevelopment, origin, false)
	if err != nil {
		log.Fatalf("rig: %s must name a loopback http origin: %v", rigBackendEnv, err)
	}
	deps.Origin = &environment
	deps.Pinned = true
	deps.Packaged = false
	deps.Browser = nil
	deps.Keychain = fake.NewKeychain()
}
