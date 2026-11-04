package app

import (
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// dictationSettingsView is the palette page for dictation: the microphone and whether the text stays on the clipboard.
func (p *App) dictationSettingsView() *palette.View {
	return &palette.View{
		Title:       "Dictation Options",
		Placeholder: "Filter dictation options",
		Search: func(q string) []search.Item {
			settings := p.dictationSettings()
			return []search.Item{
				{
					Title:     "Microphone",
					Subtitle:  "Device that records your voice",
					Icon:      "mic",
					Accessory: deviceTitle(p.dictationFlowFor().Devices(), settings.DeviceID),
					Actions: []search.Action{{Title: "Choose", Dismiss: search.DismissNever, Run: func(h search.Host) {
						h.Push(p.microphoneView())
					}}},
				},
				{
					Title:     "Copy to Clipboard",
					Subtitle:  "Keep the dictated text on the clipboard instead of pasting and restoring it",
					Icon:      "clipboard",
					Accessory: onOff(settings.CopyToClipboard),
					Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
						p.setDictationSettings(func(s *dictationSettings) { s.CopyToClipboard = !s.CopyToClipboard })
						h.Refresh()
					}}},
				},
			}
		},
	}
}

// microphoneView lists the system default and every microphone the OS offers. The list is read from the last
// listing, which the view asks for when it opens, so the palette never waits on the OS.
func (p *App) microphoneView() *palette.View {
	flow := p.dictationFlowFor()
	flow.RefreshDevices()
	return &palette.View{
		Title:       "Microphone",
		Placeholder: "Filter microphones",
		EmptyTitle:  "No microphone found",
		EmptySub:    "Connect a microphone, then open this page again",
		Search: func(q string) []search.Item {
			selected := p.dictationSettings().DeviceID
			items := []search.Item{p.microphoneRow("System Default", "", selected == "")}
			for _, device := range flow.Devices() {
				items = append(items, p.microphoneRow(device.Title, device.ID, selected == device.ID))
			}
			return items
		},
	}
}

// microphoneRow is one microphone choice; choosing it saves the device and returns to the settings page.
func (p *App) microphoneRow(title, id string, selected bool) search.Item {
	accessory := ""
	if selected {
		accessory = "Selected"
	}
	return search.Item{
		Title:     title,
		Icon:      "mic",
		Accessory: accessory,
		Actions: []search.Action{{Title: "Use", Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.setDictationSettings(func(s *dictationSettings) { s.DeviceID = id })
			h.Pop()
		}}},
	}
}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}
