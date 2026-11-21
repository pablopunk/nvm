package analytics

import (
	"crypto/sha256"
	"encoding/hex"
)

// ObserveInstallOwner records a one-way account hash once per owner per run.
func (c *Client) ObserveInstallOwner(accountID string) bool {
	if c == nil || accountID == "" || !c.accepting() {
		return false
	}
	hash := sha256.Sum256([]byte(accountID))
	ownerID := hex.EncodeToString(hash[:])

	c.ownerMu.Lock()
	defer c.ownerMu.Unlock()
	if _, observed := c.observedOwners[ownerID]; observed {
		return false
	}
	if !c.capture(EventInstallOwnerObserved, map[string]any{"owner_source": "account", "owner_id": ownerID}) {
		return false
	}
	c.observedOwners[ownerID] = struct{}{}
	return true
}
