package windowmgmt

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func rectFromPlatform(frame platform.WindowFrame) geometry.Rect {
	return geometry.Rect{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height}
}

func platformFrameOf(rect geometry.Rect) platform.WindowFrame {
	return platform.WindowFrame{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
}

func displayFromPlatform(display platform.WindowDisplay) geometry.Display {
	return geometry.Display{
		ID:       display.ID,
		Main:     display.Primary,
		Scale:    1,
		Frame:    rectFromPlatform(display.Frame),
		WorkArea: rectFromPlatform(display.WorkArea),
	}
}

func displaysFromPlatform(displays []platform.WindowDisplay) []geometry.Display {
	converted := make([]geometry.Display, 0, len(displays))
	for _, display := range displays {
		converted = append(converted, displayFromPlatform(display))
	}
	return converted
}

// windowFrameOf is the window as the frame arithmetic sees it.
func windowFrameOf(record platform.WindowRecord) geometry.WindowFrame {
	return geometry.WindowFrame{
		Frame:        rectFromPlatform(record.Frame),
		DisplayID:    record.DisplayID,
		IsResizable:  record.Resizable,
		IsFullScreen: record.FullScreen,
		IsMinimized:  record.Minimized,
	}
}

func neighborWindowOf(record platform.WindowRecord, zIndex int) geometry.NeighborWindow {
	return geometry.NeighborWindow{
		ID:           record.Key,
		Frame:        rectFromPlatform(record.Frame),
		ZIndex:       zIndex,
		DisplayID:    record.DisplayID,
		IsFullScreen: record.FullScreen,
	}
}
