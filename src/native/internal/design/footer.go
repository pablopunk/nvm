package design

import "github.com/egoist/mygo/ui"

// FooterClearance keeps fixed controls clear of the floating footer.
const FooterClearance = 48

// Footer geometry. Every footer bar is a pill of one height; the buttons inside sit one inset in from its edge, and
// the key caps inside a button sit one inset in from that, so each curve is concentric with the one around it.
const (
	FooterPillHeight   = 30
	footerInset        = 3
	footerButtonHeight = FooterPillHeight - 2*footerInset
	footerKeyInset     = 4
	footerKeyHeight    = footerButtonHeight - 2*footerKeyInset
)

// KeyHint is an action named with the key that runs it, as the footer shows it.
type KeyHint struct {
	Title string
	// Key is the key's label, already spelled for the platform. Empty draws the title alone.
	Key string
	// Emphasized makes the primary action stand out from secondary controls.
	Emphasized bool
}

// FooterData is the status bar under the palette: where the user is, and what Enter and the action menu do.
type FooterData struct {
	Name          string
	HideName      bool
	Control       func(*ui.Context)
	PrimaryAction func()
	// SecondaryAction runs on a click of the Secondary hint, which opens the action menu.
	SecondaryAction func()
	// Primary is what Enter does and Secondary what the action menu key opens. A page may have only one of them. With
	// neither, Hint, if set, is shown instead.
	Primary   *KeyHint
	Secondary *KeyHint
	Hint      string
}

// Footer draws the status bar of a view along the bottom edge of the pane.
func Footer(c *ui.Context, d FooterData) {
	k := Tokens(c)
	ui.Row(c).Absolute().Bottom(8).Left(8).Right(8).Gap(6).AlignItems(ui.Center).Children(func() {
		if !d.HideName {
			footerPill(c, k, 12).Gap(6).Children(func() {
				ui.Icon(c, Icon("zap")).FontSize(13).TextColor(k.Muted)
				ui.Text(c, d.Name).FontSize(12).TextColor(k.Muted).SingleLine()
			})
		}
		if d.Control != nil {
			footerPill(c, k, footerInset).Children(func() {
				d.Control(c)
			})
		}
		ui.Box(c).Grow(1)
		if d.Primary != nil || d.Secondary != nil {
			footerPill(c, k, footerInset).Gap(2).Children(func() {
				if d.Primary != nil {
					footerButton(c, d.Primary, d.PrimaryAction)
				}
				if d.Secondary != nil {
					footerButton(c, d.Secondary, d.SecondaryAction)
				}
			})
		} else if d.Hint != "" {
			footerPill(c, k, 12).Children(func() {
				ui.Text(c, d.Hint).FontSize(12).TextColor(k.Muted).SingleLine()
			})
		}
	})
}

// footerPill is one footer bar: a fully rounded plate with the look of the palette's active row, so the bars read as
// part of the list and not as a plain strip. Its edge is a border that takes layout space, so the padding gives it up.
func footerPill(c *ui.Context, k *TokenSet, horizontal float32) ui.Element {
	e := ui.Row(c).Height(FooterPillHeight).AlignItems(ui.Center).Radius(FooterPillHeight/2).
		Padding(0, horizontal-selectionEdgeWidth)
	return SelectedFill(e, k, false)
}

// footerButton draws a footer hint as a pill button, title then key cap, that lights up under the pointer and runs
// action on a click. With no action it keeps the same shape without the hover.
func footerButton(c *ui.Context, hint *KeyHint, action func()) {
	k := Tokens(c)
	right := float32(10)
	if hint.Key != "" {
		right = footerKeyInset
	}
	button := ui.ButtonBase(c).Height(footerButtonHeight).Radius(footerButtonHeight/2).Padding(0, right, 0, 10)
	hovered := action != nil && button.Hovered()
	if hint.Emphasized {
		Glossy(button, k.AccentFill, footerButtonHeight/2, 0)
		// Glossy always draws the edge, so changing its color on hover never shifts the layout.
		if hovered {
			button.Border(1, k.Accent)
		}
	} else if hovered {
		button.Background(k.Selected)
	}
	button.Children(func() {
		ui.Row(c).Height(footerButtonHeight).Gap(8).AlignItems(ui.Center).Children(func() {
			color := k.Strong
			if hint.Emphasized {
				color = k.AccentContrast
			}
			ui.Text(c, hint.Title).FontSize(12).FontWeight(500).TextColor(color).SingleLine()
			if hint.Key != "" {
				footerKeycap(c, hint.Key)
			}
		})
	})
	if action != nil && button.Clicked() {
		action()
	}
}
