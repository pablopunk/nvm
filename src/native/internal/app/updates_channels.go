package app

import (
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/update"
)

// updateSettingsFile holds the update channel in the user's store.
const updateSettingsFile = "updates.json"

// updateSettings are the choices the updater keeps across launches.
type updateSettings struct {
	Channel string `json:"channel"`
}

// updateChannels keeps the channel in updates.json. Without a writable user store it keeps the channel for the process.
type updateChannels struct {
	p      *App
	memory update.Channel
}

func newUpdateChannels(p *App) *updateChannels {
	return &updateChannels{p: p, memory: update.ChannelStable}
}

// Channel reads the stored channel; a file that cannot be read means stable.
func (c *updateChannels) Channel() update.Channel {
	if c.p.userStore == nil || c.p.userReadOnly {
		return c.memory
	}
	settings, err := store.Doc(c.p.userStore, updateSettingsFile, 1, func() updateSettings { return updateSettings{} }).Load()
	if err != nil {
		return update.ChannelStable
	}
	return update.ParseChannel(settings.Channel)
}

// SetChannel saves the channel.
func (c *updateChannels) SetChannel(channel update.Channel) error {
	if c.p.userStore == nil || c.p.userReadOnly {
		c.memory = channel
		return nil
	}
	return store.Doc(c.p.userStore, updateSettingsFile, 1, func() updateSettings { return updateSettings{} }).Update(func(s *updateSettings) {
		s.Channel = string(channel)
	})
}
