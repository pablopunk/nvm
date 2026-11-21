package menubar

import "github.com/pablopunk/nvm/native/internal/platform"

// ChooseGlyph picks the image of an extension's glyph name on goos with the bar dark or light, as 1.x draws a status
// item's icon: a black template that macOS tints to the bar, and elsewhere a white glyph on a dark bar.
func ChooseGlyph(name, goos string, dark bool) IconChoice {
	if goos != "darwin" && dark {
		return glyphChoice(name+"-white@2x.png", false)
	}
	return glyphChoice(name+"@2x.png", goos == "darwin")
}

func glyphChoice(file string, template bool) IconChoice {
	return IconChoice{Asset: file, PNG: iconAsset(file), Template: template}
}

// MenuGlyph is the image of a menu row with glyph name. 1.x draws row icons on macOS only, so other platforms get none.
func MenuGlyph(name, goos string) *platform.MenuIcon {
	if goos != "darwin" {
		return nil
	}
	return &platform.MenuIcon{PNG: iconAsset(name + "@2x.png"), Template: true}
}

// StatusText is the title and tooltip an item shows. Only macOS draws the title next to the icon; elsewhere the title
// joins the tooltip, as 1.x does.
func StatusText(goos, title, tooltip string) (string, string) {
	if goos == "darwin" || title == "" {
		return title, tooltip
	}
	return "", tooltip + " — " + title
}
