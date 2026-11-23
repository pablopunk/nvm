package updates

import "github.com/pablopunk/nvm/native/internal/ext/api"

// channels are the release channels, in the order the list shows them.
var channels = []string{api.UpdateChannelStable, api.UpdateChannelNightly, api.UpdateChannelDev}

// channelsView lists the channels. Choosing one switches to it, checks it, and goes back to the Updates page.
func channelsView(state api.UpdateState) api.View {
	items := make([]api.Item, 0, len(channels))
	for _, channel := range channels {
		items = append(items, channelChoice(channel, state.Channel))
	}
	view := api.ListView("Update Channel", items...)
	view.Placeholder = "Search channels"
	view.SelectedItemID = "channel-" + current(state.Channel)
	return view
}

// channelChoice is one channel of the list; the channel the updater follows is marked.
func channelChoice(channel, followed string) api.Item {
	item := api.Item{
		ID: "channel-" + channel, Title: channelLabel(channel), Subtitle: channelDescription(channel), Icon: "shield-check",
		Actions: []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Target: channel, Title: "Choose"}},
	}
	if channel == current(followed) {
		item.Accessories = []api.Accessory{{Icon: "check", Text: "Current"}}
	}
	return item
}

// current is the channel the updater follows; an unknown or empty one is stable, as the updater reads it.
func current(channel string) string {
	switch channel {
	case api.UpdateChannelNightly, api.UpdateChannelDev:
		return channel
	}
	return api.UpdateChannelStable
}

// channelLabel is the name a channel shows as.
func channelLabel(channel string) string {
	switch channel {
	case api.UpdateChannelNightly:
		return "Nightly"
	case api.UpdateChannelDev:
		return "Dev"
	}
	return "Stable"
}

// channelDescription says what a channel offers.
func channelDescription(channel string) string {
	switch channel {
	case api.UpdateChannelNightly:
		return "Nightly prereleases; a final release still wins when newer"
	case api.UpdateChannelDev:
		return "Frequent dev builds and nightly prereleases"
	}
	return "Final releases only"
}
