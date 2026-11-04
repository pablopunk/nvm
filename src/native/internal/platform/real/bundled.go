package real

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// bundledResource returns the path of a file under the app's resources, or "" when MyGo reports no resources
// directory. Packaged builds keep their natives and helpers there.
func bundledResource(parts ...string) string {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{dir}, parts...)...)
}

// existingBundledResource returns the bundled path when that file exists, else "".
func existingBundledResource(parts ...string) string {
	path := bundledResource(parts...)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}
