package design

import "github.com/egoist/mygo/ui"

// FooterClearance keeps fixed controls clear of the floating footer: a control and the inset on each side of it.
const FooterClearance = Inset + ControlHeight + Inset

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

// Footer draws the status bar of a view along the bottom edge of the pane: the context group on the left (where the user
// is and the view's value picker) and the actions group on the right (Actions, then the action Enter runs, last).
func Footer(c *ui.Context, d FooterData) {
	k := Tokens(c)
	ui.Row(c).Absolute().Bottom(Inset).Left(Inset).Right(Inset).Gap(Inset).AlignItems(ui.Center).Children(func() {
		footerContext(c, k, d)
		ui.Box(c).Grow(1)
		footerActions(c, d)
	})
}

// footerContext is the left group: the view's name and its control in one pill. It draws nothing when it has neither.
func footerContext(c *ui.Context, k *TokenSet, d FooterData) {
	showName := !d.HideName && d.Name != ""
	if !showName && d.Control == nil {
		return
	}
	footerGroup(c).Children(func() {
		if showName {
			ui.Row(c).Height(InnerHeight).Gap(6).Padding(0, ControlPad-InnerInset).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, Icon("zap")).FontSize(13).TextColor(k.Secondary)
				ui.Text(c, d.Name).FontSize(12).TextColor(k.Secondary).SingleLine()
			})
		}
		if d.Control != nil {
			d.Control(c)
		}
	})
}

// footerActions is the right group: the secondary action, then the primary one last, where Enter's action belongs. With
// neither, the view's hint takes the group.
func footerActions(c *ui.Context, d FooterData) {
	if d.Primary == nil && d.Secondary == nil {
		if d.Hint != "" {
			k := Tokens(c)
			footerGroup(c).Children(func() {
				ui.Text(c, d.Hint).FontSize(12).TextColor(k.Secondary).Padding(0, ControlPad-InnerInset).SingleLine()
			})
		}
		return
	}
	footerGroup(c).Children(func() {
		if d.Secondary != nil {
			footerButton(c, d.Secondary, d.SecondaryAction)
		}
		if d.Primary != nil {
			footerButton(c, d.Primary, d.PrimaryAction)
		}
	})
}

// footerGroup is one footer pill in the floating lens, which blurs the rows that scroll under it. Its edge is a border
// that takes layout space, so the padding gives it up and the nested pills sit one inner inset in from the curve.
func footerGroup(c *ui.Context) ui.Element {
	e := ui.Row(c).Height(ControlHeight).Radius(ControlRadius).Padding(0, InnerInset-1).Gap(2).AlignItems(ui.Center)
	return FloatingLens(e, Tokens(c))
}

// footerButton draws a footer hint as a nested pill, title then key cap, that lights up under the pointer and runs
// action on a click. The emphasized hint wears gold glass. With no action it keeps the same shape without the hover.
func footerButton(c *ui.Context, hint *KeyHint, action func()) {
	k := Tokens(c)
	right := float32(10)
	if hint.Key != "" {
		right = InnerInset
	}
	button := ui.ButtonBase(c).Height(InnerHeight).Radius(InnerRadius).Padding(0, right, 0, 10)
	hovered := action != nil && button.Hovered()
	color := k.Strong
	if hint.Emphasized {
		GoldGlass(button, k)
		color = k.GoldGlassText
		if hovered {
			button.Border(1, k.Accent)
		}
	} else if hovered {
		button.Background(k.Fill)
	}
	button.Children(func() {
		ui.Row(c).Height(InnerHeight).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, hint.Title).FontSize(12).FontWeight(500).TextColor(color).SingleLine()
			if hint.Key != "" {
				footerKeycap(c, hint.Key, color)
			}
		})
	})
	if action != nil && button.Clicked() {
		action()
	}
}
