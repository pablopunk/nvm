package menubar

import "embed"

// iconFiles holds the glyphs an item draws, copied from src/app/resources/menubar-icons at their @2x size. MyGo takes
// one PNG per icon, so the 1x files are not embedded.
//
//go:embed icons/*.png
var iconFiles embed.FS

// iconAsset returns the embedded PNG called name, or nil when no such file is embedded.
func iconAsset(name string) []byte {
	data, err := iconFiles.ReadFile("icons/" + name)
	if err != nil {
		return nil
	}
	return data
}
