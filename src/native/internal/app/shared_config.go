package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform/folders"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/update"
)

// errNoSharedConfig is returned by a write when the app runs without its config store.
var errNoSharedConfig = errors.New("settings are not available")

// sharedDir is the folder 1.x and 2.0 share. Golden tests swap it for a fixed path.
var sharedDir = defaultSharedDir

func defaultSharedDir() string {
	home, _ := os.UserHomeDir()
	return folders.Shared(home)
}

func sharedConfigPath() string {
	return filepath.Join(sharedDir(), folders.PortableConfigName)
}

// diagnosticsOff is the "Send Technical Diagnostics" switch as config.json holds it. A missing key, or a file that
// cannot be read, means diagnostics are on.
func (p *App) diagnosticsOff() bool {
	if p.sharedConfig == nil {
		return false
	}
	doc, err := p.sharedConfig.Load()
	if err != nil {
		return false
	}
	errorReporting, present := doc.Bool("settings", "errorReporting")
	return present && !errorReporting
}

func (p *App) setDiagnosticsOff(off bool) error {
	if p.sharedConfig == nil {
		return errNoSharedConfig
	}
	return p.sharedConfig.Update(func(doc portable.Document) {
		doc.SetValue(!off, "settings", "errorReporting")
	})
}

// updateChannels keeps the channel in config.json as 1.x does. Without a config store it keeps the channel for the
// process.
type updateChannels struct {
	p      *App
	memory update.Channel
}

func newUpdateChannels(p *App) *updateChannels {
	return &updateChannels{p: p, memory: update.ChannelStable}
}

// Channel reads the stored channel; a file that cannot be read means stable.
func (c *updateChannels) Channel() update.Channel {
	if c.p.sharedConfig == nil {
		return c.memory
	}
	doc, err := c.p.sharedConfig.Load()
	if err != nil {
		return update.ChannelStable
	}
	value, _ := doc.String("settings", "updateChannel")
	return update.ParseChannel(value)
}

// SetChannel saves the channel.
func (c *updateChannels) SetChannel(channel update.Channel) error {
	if c.p.sharedConfig == nil {
		c.memory = channel
		return nil
	}
	return c.p.sharedConfig.Update(func(doc portable.Document) {
		doc.SetValue(string(channel), "settings", "updateChannel")
	})
}
