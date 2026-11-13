package app

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
)

// maxDetailDiagnostics is how many of the runtime's newest diagnostics the detail pane shows.
const maxDetailDiagnostics = 5

// extensionDetailPane is the side pane of an extension row: its folder, its declared capabilities and the newest
// diagnostics the runtime kept.
func extensionDetailPane(l manager.Listing) func(c *ui.Context, _ search.Host) {
	return func(c *ui.Context, _ search.Host) { drawExtensionDetail(c, l) }
}

func drawExtensionDetail(c *ui.Context, l manager.Listing) {
	t := design.Tokens(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, extensionTitle(l)).FontSize(13).TextColor(t.Strong)
			ui.Text(c, l.ID).FontSize(11).TextColor(t.Tertiary)
			ui.Text(c, "State: "+extensionStateLabel(l)).FontSize(11).TextColor(t.Secondary)
			if reason := extensionReason(l); reason != "" {
				ui.Text(c, reason).FontSize(11).TextColor(t.Secondary)
			}
			if l.OnlyOn != "" {
				ui.Text(c, "Works on: "+l.OnlyOn).FontSize(11).TextColor(t.Secondary)
			}
			ui.Divider(c)
			drawDetailSection(c, "Folder", []string{l.Dir}, "")
			drawDetailSection(c, "Capabilities", capabilityLines(l), "None declared")
			drawDetailSection(c, "Diagnostics", newestLines(l.Runtime.Diagnostics, maxDetailDiagnostics), "No diagnostics yet")
		})
	})
}

func drawDetailSection(c *ui.Context, heading string, lines []string, empty string) {
	t := design.Tokens(c)
	ui.Text(c, heading).FontSize(11).TextColor(t.Tertiary)
	if len(lines) == 0 {
		ui.Text(c, empty).FontSize(11).TextColor(t.Secondary)
		return
	}
	for _, line := range lines {
		ui.Text(c, line).FontSize(11).TextColor(t.Primary)
	}
}

// capabilityLines names each declared capability, marking the ones this version does not know.
func capabilityLines(l manager.Listing) []string {
	lines := make([]string, 0, len(l.Capabilities))
	for _, capability := range l.Capabilities {
		line := string(capability)
		if slices.Contains(l.UnknownCapabilities, capability) {
			line += " (unknown)"
		}
		lines = append(lines, line)
	}
	return lines
}

// newestLines keeps the last n lines of a log, which are the newest.
func newestLines(lines []string, n int) []string {
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}
