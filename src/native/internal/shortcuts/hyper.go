package shortcuts

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/portable"
)

// DefaultHyperKey is the Hyper key of a new installation, as 1.x sets it.
const DefaultHyperKey = "Command+Control+Alt+Shift"

// HyperSymbol stands in front of a label whose shortcut starts with the Hyper key: ✦D is Hyper+D.
const HyperSymbol = "✦"

// HyperKeyOf reads the Hyper key of config.json. A missing key is the default. A stored key is used as written, even
// when it is blank, as 1.x does.
func HyperKeyOf(stored portable.Document) string {
	if hyper := stored.Settings().HyperKey; hyper != nil {
		return *hyper
	}
	return DefaultHyperKey
}

// Labeler spells accelerators for display: the key names of GOOS, and the Hyper key ("" for none). Matching never uses
// a label, so the modifiers of a shortcut stay the real ones.
type Labeler struct {
	GOOS  string
	Hyper string
}

// Label spells accelerator for the platform. An accelerator that starts with the Hyper key shows ✦ in place of those
// keys: Command+Control+Alt+D with Command+Control+Alt as Hyper reads ✦D.
func (l Labeler) Label(accelerator string) string {
	if rest, ok := StripHyper(accelerator, l.Hyper); ok {
		return HyperSymbol + Label(rest, l.GOOS)
	}
	return Label(accelerator, l.GOOS)
}

// Keycaps spells accelerator as one cap per key: ✦ for the Hyper key, then each key the shortcut still names.
func (l Labeler) Keycaps(accelerator string) []string {
	rest, hyper := StripHyper(accelerator, l.Hyper)
	var caps []string
	if hyper {
		caps = append(caps, HyperSymbol)
	} else {
		rest = accelerator
	}
	for _, part := range Parts(rest) {
		caps = append(caps, Label(part, l.GOOS))
	}
	return caps
}

// StripHyper returns accelerator without the Hyper key in front of it. ok is false when accelerator does not start with
// every part of hyper, in order, or when hyper is blank.
func StripHyper(accelerator, hyper string) (rest string, ok bool) {
	hyperParts := Parts(hyper)
	parts := Parts(accelerator)
	if len(hyperParts) == 0 || len(parts) < len(hyperParts) || !slices.Equal(parts[:len(hyperParts)], hyperParts) {
		return "", false
	}
	return strings.Join(parts[len(hyperParts):], "+"), true
}
