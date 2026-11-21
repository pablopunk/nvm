package x11

// Extents are the decoration widths a window manager adds around a client window, from _NET_FRAME_EXTENTS.
type Extents struct {
	Left, Right, Top, Bottom int
}

// Framed returns the frame that surrounds a client rectangle. The engine works in frames, so readbacks
// are converted with this.
func (r Rect) Framed(e Extents) Rect {
	return Rect{
		X:      r.X - e.Left,
		Y:      r.Y - e.Top,
		Width:  r.Width + e.Left + e.Right,
		Height: r.Height + e.Top + e.Bottom,
	}
}

// Client returns the client rectangle inside a frame rectangle. Client sizes stay at least one pixel.
func (r Rect) Client(e Extents) Rect {
	return Rect{
		X:      r.X + e.Left,
		Y:      r.Y + e.Top,
		Width:  max(1, r.Width-e.Left-e.Right),
		Height: max(1, r.Height-e.Top-e.Bottom),
	}
}

// Extents decodes a _NET_FRAME_EXTENTS value (left, right, top, bottom). An unset property means no decoration.
func (p Properties) Extents(name string) Extents {
	values := p.Cardinals(name)
	if len(values) < 4 {
		return Extents{}
	}
	return Extents{Left: values[0], Right: values[1], Top: values[2], Bottom: values[3]}
}
