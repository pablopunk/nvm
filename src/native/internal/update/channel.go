package update

// Channel is the release line the updater follows. Stable offers only final releases. Nightly also offers the
// prereleases, and a final release still wins whenever it is newer. The choice is persisted by the app.
type Channel string

// The channels a person can choose.
const (
	ChannelStable  Channel = "stable"
	ChannelNightly Channel = "nightly"
)

// ParseChannel reads a stored channel. Anything unknown is stable, the default.
func ParseChannel(value string) Channel {
	if Channel(value) == ChannelNightly {
		return ChannelNightly
	}
	return ChannelStable
}

// Label is the name the Settings row shows.
func (c Channel) Label() string {
	if c == ChannelNightly {
		return "Nightly"
	}
	return "Stable"
}

// Prereleases reports whether the channel offers prerelease versions.
func (c Channel) Prereleases() bool {
	return c == ChannelNightly
}

// Other is the channel the Settings row switches to.
func (c Channel) Other() Channel {
	if c == ChannelNightly {
		return ChannelStable
	}
	return ChannelNightly
}
