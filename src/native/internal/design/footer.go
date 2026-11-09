package design

import "github.com/egoist/mygo/ui"

// FooterClearance keeps fixed controls clear of the floating footer.
const FooterClearance = 48

// KeyHint is an action named with the key that runs it, as the footer shows it.
type KeyHint struct {
	Title string
	// Key is the key's label, already spelled for the platform. Empty draws the title alone.
	Key string
}

// FooterData is the status bar under the palette: where the user is, and what Enter and the action menu do.
type FooterData struct {
	Name          string
	HideName      bool
	Control       func(*ui.Context)
	PrimaryAction func()
	// Primary is what Enter does and Secondary what the action menu key opens. A page may have only one of them. With
	// neither, Hint, if set, is shown instead.
	Primary   *KeyHint
	Secondary *KeyHint
	Hint      string
}

// Footer draws the status bar of a view along the bottom edge of the pane.
func Footer(c *ui.Context, d FooterData) {
	k := Tokens(c)
	ui.Row(c).Absolute().Bottom(8).Left(8).Right(8).Gap(8).AlignItems(ui.Center).Children(func() {
		if !d.HideName {
			footerPlate(ui.Row(c).Gap(6).AlignItems(ui.Center).Radius(16), k, 6, 10).Children(func() {
				ui.Icon(c, Icon("zap")).FontSize(14).TextColor(k.Muted)
				ui.Text(c, d.Name).FontSize(12).TextColor(k.Muted).SingleLine()
			})
		}
		if d.Control != nil {
			footerPlate(ui.Row(c).Radius(16), k, 3, 4).Children(func() {
				d.Control(c)
			})
		}
		ui.Box(c).Grow(1)
		if d.Primary != nil || d.Secondary != nil || d.Hint != "" {
			footerPlate(ui.Row(c).Gap(12).AlignItems(ui.Center).Radius(16), k, 6, 10).Children(func() {
				if d.Primary != nil {
					if d.PrimaryAction != nil {
						button := ui.ButtonBase(c)
						button.Children(func() { keyHint(c, d.Primary) })
						if button.Clicked() {
							d.PrimaryAction()
						}
					} else {
						keyHint(c, d.Primary)
					}
				}
				if d.Secondary != nil {
					keyHint(c, d.Secondary)
				}
				if d.Primary == nil && d.Secondary == nil && d.Hint != "" {
					ui.Text(c, d.Hint).FontSize(12).TextColor(k.Muted).SingleLine()
				}
			})
		}
	})
}

// footerPlate gives a footer bar the look of the palette's active row: the same lit plate and edge, so the bars read as
// part of the list and not as a plain strip.
func footerPlate(e ui.Element, k *TokenSet, vertical, horizontal float32) ui.Element {
	return SelectedFill(PlatePadding(e, true, vertical, horizontal), k, false)
}

// keyHint draws an action's title with its key cap at the end of the footer.
func keyHint(c *ui.Context, hint *KeyHint) {
	k := Tokens(c)
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, hint.Title).FontSize(12).FontWeight(500).TextColor(k.Strong)
		if hint.Key != "" {
			Keycap(c, hint.Key)
		}
	})
}
