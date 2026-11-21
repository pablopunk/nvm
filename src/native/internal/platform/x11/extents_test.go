package x11

import "testing"

func TestFrameAndClientConvertWithDecorationExtents(t *testing.T) {
	decorated := Extents{Left: 1, Right: 1, Top: 20, Bottom: 1}
	frame := Rect{X: 0, Y: 27, Width: 640, Height: 773}

	client := frame.Client(decorated)
	if client != (Rect{X: 1, Y: 47, Width: 638, Height: 752}) {
		t.Fatalf("Client = %+v", client)
	}
	if back := client.Framed(decorated); back != frame {
		t.Fatalf("Framed(Client(frame)) = %+v, want %+v", back, frame)
	}
}

func TestClientSizeNeverDropsBelowOnePixel(t *testing.T) {
	client := Rect{Width: 2, Height: 2}.Client(Extents{Left: 5, Right: 5, Top: 5, Bottom: 5})
	if client.Width != 1 || client.Height != 1 {
		t.Fatalf("Client = %+v, want one pixel", client)
	}
}

func TestExtentsDecodeFromTheDesktopProperty(t *testing.T) {
	properties := ParseProperties("_NET_FRAME_EXTENTS(CARDINAL) = 1, 1, 20, 1")
	if got := properties.Extents("_NET_FRAME_EXTENTS"); got != (Extents{Left: 1, Right: 1, Top: 20, Bottom: 1}) {
		t.Fatalf("Extents = %+v", got)
	}
	if got := ParseProperties("_NET_FRAME_EXTENTS: not found.").Extents("_NET_FRAME_EXTENTS"); got != (Extents{}) {
		t.Fatalf("unset extents = %+v, want none", got)
	}
}
