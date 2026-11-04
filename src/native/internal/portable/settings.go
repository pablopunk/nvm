package portable

// Settings are the portable settings 1.x and 2.0 share, read from the settings object. A key that is missing, or holds
// another type, leaves its field at the zero value. ErrorReporting is nil when the key is missing, so the reader can
// apply its own default.
type Settings struct {
	PaletteHotkey    string
	HyperKey         string
	UpdateChannel    string
	MenuBarIconStyle string
	ErrorReporting   *bool
	AIDefaultModel   string
}

// Settings reads the portable settings of the document.
func (d Document) Settings() Settings {
	var s Settings
	s.PaletteHotkey, _ = d.String("settings", "paletteHotkey")
	s.HyperKey, _ = d.String("settings", "hyperKey")
	s.UpdateChannel, _ = d.String("settings", "updateChannel")
	s.MenuBarIconStyle, _ = d.String("settings", "menuBarIconStyle")
	if flag, present := d.Bool("settings", "errorReporting"); present {
		s.ErrorReporting = &flag
	}
	s.AIDefaultModel, _ = d.String("ai", "defaultModel")
	return s
}
