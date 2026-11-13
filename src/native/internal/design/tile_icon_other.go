//go:build !darwin

package design

// FileIconSize is the side a file or app icon is drawn at. Linux and Windows icons fill their canvas, so they are
// drawn at the tile's own size to match a TileSize key beside them.
const FileIconSize = TileSize
