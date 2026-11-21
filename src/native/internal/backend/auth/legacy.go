package auth

import (
	"encoding/json"
	"fmt"
)

// legacyDocument is the account document as the earlier 2.0 builds wrote it: the store's envelope around the Profile.
type legacyDocument struct {
	SchemaVersion int     `json:"schemaVersion"`
	Data          Profile `json:"data"`
}

// DecodeLegacyProfile reads an account.json that an earlier build wrote. Fields it lacks keep their defaults, and a
// document from a newer schema is refused, since this build cannot read it.
func DecodeLegacyProfile(raw []byte) (Profile, error) {
	doc := legacyDocument{Data: DefaultProfile()}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Profile{}, fmt.Errorf("auth: the earlier account document is not readable: %w", err)
	}
	if doc.SchemaVersion < 1 || doc.SchemaVersion > ProfileSchemaVersion {
		return Profile{}, fmt.Errorf("auth: the earlier account document has schema version %d", doc.SchemaVersion)
	}
	return doc.Data, nil
}
