package extview

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// escapeFooterKey is the key a view's footer names for the Escape key, which no accelerator spells.
const escapeFooterKey = "Escape"

// footerHintsText spells a view's footer keys as the palette shows them, three spaces apart.
func footerHintsText(hints []api.FooterHint, hyper string) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, footerKeyLabel(hint.Key, hyper)+" "+hint.Title)
	}
	return strings.Join(parts, "   ")
}

// footerKeyLabel spells one footer key: Escape as "esc", and an accelerator the way the palette spells shortcuts.
func footerKeyLabel(key, hyper string) string {
	if key == escapeFooterKey {
		return "esc"
	}
	return shortcutLabel(key, hyper)
}
