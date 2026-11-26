package design

// Geometry. Three numbers lay out every control on the pane: Inset is the space between the pane edge and a control and
// between two controls, ControlHeight is the height of every pill, field and bubble, and ControlPad is the inset of the
// text in a control. Radii are concentric: a curve one inset in from another has that much less radius.
const (
	Inset         = 8
	ControlHeight = 32
	ControlRadius = ControlHeight / 2
	ControlPad    = 14
	// InnerInset is the space between a group pill and the pills nested in it, and between a nested pill and its key cap.
	InnerInset  = 4
	InnerHeight = ControlHeight - 2*InnerInset
	InnerRadius = ControlRadius - InnerInset
	KeyHeight   = InnerHeight - 2*InnerInset
	KeyRadius   = InnerRadius - InnerInset
	// ReadingInset is the inset of the text on a page that is read rather than chosen from, such as a preview: a
	// control's inset from the pane plus the text's inset in that control, so the text lines up with the footer's
	// leading content and with the text of an editor.
	ReadingInset = Inset + ControlPad
)

// PaneRadius is the corner radius of the palette window: a control's radius plus the inset around it, so the pane's
// curve is concentric with the controls along its edge. The window cut and the clip both use it.
const PaneRadius = ControlRadius + Inset
