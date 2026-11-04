package app

import "github.com/pablopunk/nvm/native/internal/search"

// launchAtLoginRows is the Launch at Login row of the settings, present only where the OS can start the app at
// sign-in. A failed change is explained in a toast; the row then reads the state the OS really has.
func (p *App) launchAtLoginRows() []search.Item {
	if !p.launchAtLogin.Supported() {
		return nil
	}
	state := "Off"
	if p.launchAtLogin.Enabled() {
		state = "On"
	}
	return []search.Item{{Title: "Launch at Login", Accessory: state, Icon: "power", Actions: []search.Action{{
		Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
			message, _ := p.launchAtLogin.Set(!p.launchAtLogin.Enabled())
			h.Toast(message)
			h.Refresh()
		},
	}}}}
}
