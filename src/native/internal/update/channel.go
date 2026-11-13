package update

// Channel is the release line the updater follows. Stable offers only final releases. Nightly and Dev also offer the
// prereleases, and a final release still wins whenever it is newer. The choice is persisted by the app.
type Channel string

// The channels a person can choose, in the order the Settings row cycles through them, as 1.x lists them.
const (
	ChannelStable  Channel = "stable"
	ChannelNightly Channel = "nightly"
	ChannelDev     Channel = "dev"
)

// ParseChannel reads a stored channel. Anything unknown is stable, the default.
func ParseChannel(value string) Channel {
	switch Channel(value) {
	case ChannelNightly:
		return ChannelNightly
	case ChannelDev:
		return ChannelDev
	}
	return ChannelStable
}

// Label is the name the Settings row shows.
func (c Channel) Label() string {
	switch c {
	case ChannelNightly:
		return "Nightly"
	case ChannelDev:
		return "Dev"
	}
	return "Stable"
}

// Prereleases reports whether the channel offers prerelease versions.
func (c Channel) Prereleases() bool {
	return c == ChannelNightly || c == ChannelDev
}

// Next is the channel the Settings row switches to: Stable, then Nightly, then Dev, then Stable again.
func (c Channel) Next() Channel {
	switch c {
	case ChannelStable:
		return ChannelNightly
	case ChannelNightly:
		return ChannelDev
	}
	return ChannelStable
}
