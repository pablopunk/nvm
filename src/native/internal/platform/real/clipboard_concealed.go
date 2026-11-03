package real

import "strings"

// concealedMarkers are substrings of platform clipboard format names that password managers set to ask
// clipboard tools not to record a copy: the macOS nspasteboard convention and the Windows
// exclude-from-history flag.
var concealedMarkers = []string{"concealed", "excludeclipboardcontent"}

// isConcealedClipboard reports whether any format name carries a concealed marker.
func isConcealedClipboard(formats []string) bool {
	for _, format := range formats {
		lower := strings.ToLower(format)
		for _, marker := range concealedMarkers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}
