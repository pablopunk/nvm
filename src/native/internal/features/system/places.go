package system

import "path/filepath"

// Place is a folder of the user's home that the palette opens.
type Place struct {
	Name string
	Path string
}

// placeNames are the home folders the palette offers, in the order it lists them.
var placeNames = []string{"Downloads", "Documents", "Desktop", "Pictures"}

// Places returns the standard folders of the home directory. An empty home yields none, so no relative path is
// ever opened.
func Places(home string) []Place {
	if home == "" {
		return nil
	}
	places := make([]Place, 0, len(placeNames))
	for _, name := range placeNames {
		places = append(places, Place{Name: name, Path: filepath.Join(home, name)})
	}
	return places
}
