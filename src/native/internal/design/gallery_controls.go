package design

import "github.com/egoist/mygo/ui"

// galleryControls shows the small controls: keys, chips, the search row, the action menu, a popover and the footer.
func galleryControls(c *ui.Context) {
	Section(c, "Keycap")
	ui.Row(c).Gap(6).Children(func() {
		Keycap(c, "↵")
		Keycap(c, "⌃K")
		Keycap(c, "⌃⌫")
	})
	Section(c, "Chip")
	ui.Row(c).Gap(6).Children(func() {
		Chip(c, ChipData{Icon: "sparkles", Bold: "Tab", Text: "to ask AI"})
		Chip(c, ChipData{Text: "Plain chip"})
	})
	Section(c, "Select: a popup button, current and disabled")
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		Select(c, SelectData{Options: []string{"Fast", "Smart"}, Selected: 0})
		Select(c, SelectData{Options: []string{"Fast", "Smart"}, Selected: 1, Disabled: true})
	})
	Section(c, "SearchRow: typed, empty, and a fixed title with back")
	SearchRow(c, SearchData{Placeholder: "Search Clipboard History", Query: &galleryState.query, Input: &galleryState.searchInput, AIHint: true})
	SearchRow(c, SearchData{Placeholder: "Watcha gonna do?", Query: &galleryState.blank, Input: &galleryState.emptyInput})
	SearchRow(c, SearchData{HideInput: true, Title: "Clipboard History", Back: true})
	Section(c, "ActionMenu: a selected action, one with a key, one with the return glyph")
	ui.Column(c).Height(200).Children(func() {
		ActionMenu(c, []MenuRow{
			{Title: "Copy to Clipboard", Enter: true},
			{Title: "Paste", Key: "⌃↵", Selected: true},
			{Title: "Pin", Key: "⌃P"},
			{Title: "Delete", Key: "⌃⌫"},
		})
	})
	Section(c, "Popover: a raised surface over the pane")
	ui.Column(c).Height(120).Children(func() {
		Popover(c, ActionPlacement, func() {
			ui.Text(c, "A popover holds any rows the action menu does not.").FontSize(12).SingleLine()
		})
	})
	Section(c, "Footer: with actions, and with a hint")
	ui.Column(c).Height(FooterClearance).Children(func() {
		Footer(c, FooterData{Name: "Clipboard History", Primary: &KeyHint{Title: "Copy to Clipboard", Key: "↵"}, Secondary: &KeyHint{Title: "Actions", Key: "⌃K"}})
	})
	ui.Column(c).Height(FooterClearance).Children(func() {
		Footer(c, FooterData{Name: "Settings", Hint: "esc Back"})
	})
}
