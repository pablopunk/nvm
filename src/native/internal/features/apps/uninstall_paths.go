package apps

import "path"

// Where an app's own data lives, per platform. Every location is derived from the home folder and the identifier,
// and each entry names the folder it must stay inside (its root). Nothing here is a wildcard: name matching is a
// separate, unchecked suggestion that macOS shows and Linux does not offer.

// minNameMatchLength is the shortest app name that may match a folder by name; shorter names match too much.
const minNameMatchLength = 3

// locationSpec is one place to look for an app's data: the folder, the root it must stay in, and the slot name.
type locationSpec struct {
	Slot string
	Path string
	Root string
	Kind string
}

// patternRoot is a folder scanned for names that start with the identifier (or equal the app name, where allowed).
type patternRoot struct {
	Slot            string
	Root            string
	AllowsNameMatch bool
}

func (u *Uninstaller) userApplicationsRoot() string {
	return path.Join(u.deps.Home, "Applications")
}

func (u *Uninstaller) library() string {
	return path.Join(u.deps.Home, "Library")
}

func (u *Uninstaller) userDesktopRoot() string {
	return path.Join(u.deps.Home, ".local", "share", "applications")
}

func (u *Uninstaller) flatpakExportRoot() string {
	return path.Join(u.deps.Home, ".local", "share", "flatpak", "exports", "share", "applications")
}

// systemDesktopRoots are where packages install desktop entries for every user. Uninstall never touches them.
var systemDesktopRoots = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
	"/var/lib/flatpak/exports/share/applications",
}

// exactSpecsDarwin are the macOS locations keyed by the exact bundle identifier.
func (u *Uninstaller) exactSpecsDarwin(bundleID string) []locationSpec {
	lib := u.library()
	return []locationSpec{
		{"application-support", path.Join(lib, "Application Support", bundleID), path.Join(lib, "Application Support"), KindAssociated},
		{"caches", path.Join(lib, "Caches", bundleID), path.Join(lib, "Caches"), KindAssociated},
		{"preferences", path.Join(lib, "Preferences", bundleID+".plist"), path.Join(lib, "Preferences"), KindAssociated},
		{"saved-state", path.Join(lib, "Saved Application State", bundleID+".savedState"), path.Join(lib, "Saved Application State"), KindAssociated},
		{"containers", path.Join(lib, "Containers", bundleID), path.Join(lib, "Containers"), KindAssociated},
		{"application-scripts", path.Join(lib, "Application Scripts", bundleID), path.Join(lib, "Application Scripts"), KindAssociated},
		{"http-storages", path.Join(lib, "HTTPStorages", bundleID), path.Join(lib, "HTTPStorages"), KindAssociated},
		{"webkit", path.Join(lib, "WebKit", bundleID), path.Join(lib, "WebKit"), KindAssociated},
		{"cookies", path.Join(lib, "Cookies", bundleID+".binarycookies"), path.Join(lib, "Cookies"), KindAssociated},
	}
}

// patternRootsDarwin are the macOS folders scanned for prefix and name matches.
func (u *Uninstaller) patternRootsDarwin() []patternRoot {
	lib := u.library()
	return []patternRoot{
		{"application-support", path.Join(lib, "Application Support"), true},
		{"caches", path.Join(lib, "Caches"), true},
		{"preferences", path.Join(lib, "Preferences"), false},
		{"saved-state", path.Join(lib, "Saved Application State"), false},
		{"containers", path.Join(lib, "Containers"), false},
		{"http-storages", path.Join(lib, "HTTPStorages"), false},
		{"webkit", path.Join(lib, "WebKit"), false},
	}
}

// exactSpecsLinux are the per-user locations of a desktop application: its data folder under the XDG data home,
// and for a flatpak its install and its app data. Each is keyed by the exact desktop identifier.
func (u *Uninstaller) exactSpecsLinux(id string, flatpak bool) []locationSpec {
	share := path.Join(u.deps.Home, ".local", "share")
	specs := []locationSpec{
		{"user-data", path.Join(share, id), share, KindAssociated},
	}
	if flatpak {
		specs = append(specs,
			locationSpec{"flatpak-install", path.Join(share, "flatpak", "app", id), path.Join(share, "flatpak", "app"), KindAssociated},
			locationSpec{"flatpak-data", path.Join(u.deps.Home, ".var", "app", id), path.Join(u.deps.Home, ".var", "app"), KindAssociated},
		)
	}
	return specs
}

// patternMatch is how a folder name matched: a prefix of the identifier, or the app's name.
type patternMatch string

const (
	matchPrefix patternMatch = "prefix"
	matchName   patternMatch = "name"
)

// matchFolderName reports whether a folder name belongs to the app by its identifier or by its name, ignoring case.
// Name matches are allowed only in folders that the 1.x layout allows them in, and only for names of enough length.
func matchFolderName(entry, bundleID, appName string, allowsName bool) (patternMatch, bool) {
	lower := lowerASCII(entry)
	if len(lower) > len(bundleID) && lower[:len(bundleID)+1] == lowerASCII(bundleID)+"." {
		return matchPrefix, true
	}
	if allowsName && len(appName) >= minNameMatchLength && lower == lowerASCII(appName) {
		return matchName, true
	}
	return "", false
}

// lowerASCII lowercases ASCII letters and leaves every other byte as it is, so it never changes a path's length.
func lowerASCII(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + 'a' - 'A'
		}
	}
	return string(out)
}
