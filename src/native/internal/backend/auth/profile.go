package auth

import (
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/store"
)

// ProfileSchemaVersion is the version of the account document. Bump it with a migration when a field changes
// meaning; a file written by a newer build is read as defaults and left alone.
const ProfileSchemaVersion = 1

// profileFileName is the store document that holds the Profile.
const profileFileName = "account.json"

// ProfileDocument opens the account document in st.
func ProfileDocument(st *store.Store) *store.Collection[Profile] {
	return store.Doc(st, profileFileName, ProfileSchemaVersion, DefaultProfile)
}

// Profile is the non-secret account state, stored as account.json. It holds no token and no key: those are in the
// keychain (see Vault).
type Profile struct {
	// Environment is the backend the app talks to.
	Environment backend.EnvironmentKind `json:"environment"`
	// BaseURL is the origin of that backend. Empty means the environment's default origin.
	BaseURL string `json:"baseUrl"`
	// Connections holds the account that is signed in for each origin.
	Connections map[string]Connection `json:"connections"`
	// BYO describes the bring-your-own provider key, without the key.
	BYO *BYOSetting `json:"byo,omitempty"`
}

// Connection is the account identity signed in for one origin. The token is in the keychain.
type Connection struct {
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	ConnectedAt string `json:"connectedAt"`
}

// BYOSetting describes the provider the user brings their own key for.
type BYOSetting struct {
	// Provider names the provider, such as "openai".
	Provider string `json:"provider"`
	// ModelID is the model the key is used with.
	ModelID string `json:"modelId"`
	// BaseURL is the origin requests are sent to.
	BaseURL string `json:"baseUrl"`
	// SavedAt is when the key was last set.
	SavedAt string `json:"savedAt"`
}

// DefaultProfile is the profile of a fresh install: the production backend, nobody signed in.
func DefaultProfile() Profile {
	return Profile{
		Environment: backend.EnvironmentProduction,
		Connections: map[string]Connection{},
	}
}
