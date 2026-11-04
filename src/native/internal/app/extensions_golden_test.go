package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// extensionGoldens is the group of the extension goldens under testdata/golden/extensions.
var extensionGoldens = golden.Options{Group: "extensions", Width: width, Height: height}

// goldenExtensionListings holds one extension in each main state the list shows, in the order the manager lists them.
func goldenExtensionListings() []manager.Listing {
	return []manager.Listing{goodListing(), brokenListing(), crashListing(), spinListing()}
}

// goldenBrokenRow is the row index of the broken extension in the list: the two commands come first. The golden
// chooses it so the detail pane shows an extension's folder, reason and diagnostics.
const goldenBrokenRow = 3

func TestPaletteExtensionManagerList(t *testing.T) {
	p := goldenPalette(t)
	ctl := &fakeExtensions{listings: goldenExtensionListings()}
	goldenOpen(p, p.extensionsViewFor(fixedController(ctl)))
	p.Choose(goldenBrokenRow)
	golden.AssertMatrix(t, "manager-list", p.View, extensionGoldens)
}

func TestPaletteExtensionCapabilityReview(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.extensionReviewView(&fakeExtensions{}, reviewListing()))
	golden.AssertMatrix(t, "capability-review", p.View, extensionGoldens)
}
