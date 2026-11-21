package analytics

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
)

// installIDFile is the file in the config directory that holds the install identifier.
const installIDFile = "analytics-id"

// loadInstallID reads the persisted identifier, or creates and saves one the first time. One identifier per install,
// whether or not anyone is signed in, so a device's history never splits. An unwritable directory only makes this
// run's identifier ephemeral.
func (c *Client) loadInstallID() {
	if c.installID != "" {
		return
	}
	path := filepath.Join(c.cfg.Dir, installIDFile)
	if data, err := c.cfg.FS.ReadFile(path); err == nil {
		if stored := strings.TrimSpace(string(data)); stored != "" {
			c.installID = stored
			return
		}
	}
	c.installID = newUUID()
	_ = c.cfg.FS.MkdirAll(c.cfg.Dir, 0o700)
	if err := c.cfg.FS.WriteFile(path, []byte(c.installID), 0o600); err != nil {
		c.warn("analytics.id.unsaved", err)
	}
}

// newUUID returns a random version 4 UUID from the operating system's generator.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
