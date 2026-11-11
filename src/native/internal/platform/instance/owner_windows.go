//go:build windows

package instance

import "os"

// ownedPrivately trusts the folder on Windows, where the temporary folder is already the user's own profile folder.
func ownedPrivately(os.FileInfo, int) bool { return true }
