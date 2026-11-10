package menubar

// DefaultIconStyle is the menuBarIconStyle an item draws when the setting is empty or unknown. It is 1.x's default.
const DefaultIconStyle = "bolt-filled"

// iconStyle is one menuBarIconStyle value: the glyph it draws, whether macOS tints it to the bar, and whether
// macOS shows it as the emoji title instead of an image.
type iconStyle struct {
	value      string
	glyph      string
	template   bool
	emojiTitle bool
}

// iconStyles mirrors APP_MENU_BAR_ICON_STYLES in src/app/shared/menubar-icons.ts, in the same order.
var iconStyles = []iconStyle{
	{value: "bolt-filled", glyph: "bolt-filled", template: true},
	{value: "bolt-outline", glyph: "bolt-outline", template: true},
	{value: "emoji", glyph: "emoji", emojiTitle: true},
	{value: "app-icon", glyph: "app-icon"},
	{value: "gold-outline", glyph: "gold-outline"},
	{value: "gold-filled", glyph: "gold-filled"},
	{value: "gold-glossy", glyph: "gold-glossy"},
}

// NormalizeIconStyle returns style when it is a known menuBarIconStyle value, and DefaultIconStyle otherwise.
func NormalizeIconStyle(style string) string {
	return resolveIconStyle(style).value
}

// resolveIconStyle returns the style's definition, or the default's when style is unknown.
func resolveIconStyle(style string) iconStyle {
	if found, ok := lookupIconStyle(style); ok {
		return found
	}
	fallback, _ := lookupIconStyle(DefaultIconStyle)
	return fallback
}

func lookupIconStyle(style string) (iconStyle, bool) {
	for _, candidate := range iconStyles {
		if candidate.value == style {
			return candidate, true
		}
	}
	return iconStyle{}, false
}
