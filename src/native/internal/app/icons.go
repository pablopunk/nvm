package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// iconSize is the size in DIPs of the system icon a row shows; the icon is decoded at twice that for
// high-density screens.
const iconSize = 28

// fileIcon returns the system icon of a file, decoded once and kept. The cache is cleared when it grows past
// 400 entries, which bounds its memory. A file without an icon is remembered as nil.
func (p *App) fileIcon(path string) *ui.Bitmap {
	if b, ok := p.icons[path]; ok {
		return b
	}
	if len(p.icons) > 400 {
		p.icons = map[string]*ui.Bitmap{}
	}
	var bm *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(path, iconSize*2); err == nil {
		bm, _ = ui.DecodeBitmap(data)
	}
	p.icons[path] = bm
	return bm
}
