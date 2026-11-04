package app

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

const (
	capabilitySection = "Declared capabilities"
	warningSection    = "Warning"
)

// extensionReviewView is the review an extension gets before its first enable. Enable calls the manager and
// returns to the list; Cancel returns and leaves the extension off.
func (p *App) extensionReviewView(ctl extensionController, l manager.Listing) *palette.View {
	return &palette.View{
		Title: "Review " + extensionTitle(l), Placeholder: "Review extension",
		Search: func(string) []search.Item { return extensionReviewRows(ctl, l) },
	}
}

func extensionReviewRows(ctl extensionController, l manager.Listing) []search.Item {
	rows := []search.Item{
		{Section: l.ID, Title: "Enable", Subtitle: "Starts it now and keeps it on after restart", Icon: "power", Tone: "green", Prominent: true,
			Actions: []search.Action{{Title: "Enable", Dismiss: search.DismissNever, Run: func(h search.Host) {
				enableFromReview(ctl, l.ID, h)
			}}}},
		{Title: "Cancel", Icon: "x", Tone: "gray",
			Actions: []search.Action{{Title: "Cancel", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }}}},
	}
	rows = append(rows, extensionCapabilityRows(l)...)
	return append(rows, extensionReviewWarning())
}

// enableFromReview returns to the list before it reports a failure, so the message lands where the row is.
func enableFromReview(ctl extensionController, id string, h search.Host) {
	err := ctl.Enable(id)
	h.Pop()
	if err != nil {
		h.Toast(err.Error())
	}
}

// extensionCapabilityRows lists what the extension declares. An unknown capability is shown as such, never hidden.
func extensionCapabilityRows(l manager.Listing) []search.Item {
	if len(l.Capabilities) == 0 {
		return []search.Item{{Section: capabilitySection, Title: "None declared", Subtitle: "The extension did not say what it does", Icon: "lock", Tone: "gray"}}
	}
	rows := make([]search.Item, 0, len(l.Capabilities))
	for i, capability := range l.Capabilities {
		row := search.Item{Title: string(capability), Subtitle: "Declared capability", Icon: "lock", Tone: "blue"}
		if i == 0 {
			row.Section = capabilitySection
		}
		if slices.Contains(l.UnknownCapabilities, capability) {
			row.Subtitle, row.Accessory, row.Tone = "Unknown to this version of Nevermind", "Unknown", "orange"
		}
		rows = append(rows, row)
	}
	return rows
}

func extensionReviewWarning() search.Item {
	return search.Item{
		Section: warningSection, Title: "Capabilities are review metadata, not a sandbox",
		Subtitle: "Enabling gives full access to this computer. See docs/extension-trust-model.md",
		Icon:     "lock", Tone: "orange",
	}
}
