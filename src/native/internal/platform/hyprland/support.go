package hyprland

// Format is the config language of the entry file Hyprland loads.
type Format string

const (
	// FormatConf is hyprlang, hyprland.conf.
	FormatConf Format = "conf"
	// FormatLua is the Lua config, hyprland.lua, from 0.55.
	FormatLua Format = "lua"
)

// Extension is the file extension of the format.
func (f Format) Extension() string { return "." + string(f) }

// The version boundaries of the support table in doc.go.
var (
	// MinimumVersion is the oldest release whose bind and hyprctl JSON syntax this package writes and reads.
	MinimumVersion = Version{0, 45, 0}
	// windowRulesV3 is the release whose hyprlang window rules take match: props (rule syntax v3).
	windowRulesV3 = Version{0, 53, 0}
	// luaConfig is the release that reads hyprland.lua and deprecates hyprlang.
	luaConfig = Version{0, 55, 0}
)

// Support is what this package can write for one version and format.
type Support struct {
	Binds       bool
	WindowRules bool
}

// SupportFor reports what can be written for a version in a format.
func SupportFor(v Version, f Format) Support {
	if !v.AtLeast(MinimumVersion) {
		return Support{}
	}
	switch f {
	case FormatLua:
		return Support{Binds: v.AtLeast(luaConfig), WindowRules: v.AtLeast(luaConfig)}
	case FormatConf:
		return Support{Binds: true, WindowRules: v.AtLeast(windowRulesV3)}
	}
	return Support{}
}
