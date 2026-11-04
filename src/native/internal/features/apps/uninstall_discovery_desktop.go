package apps

import (
	"path"
	"strings"
)

// systemPackageMessage explains why a system-wide app is not offered, and how to remove it instead.
const systemPackageMessage = "This app is installed for every user by your system’s package manager. Nevermind only moves apps you installed for yourself to Trash. Remove it with that package manager (apt, dnf, pacman or zypper), or with flatpak uninstall --system for a system flatpak."

// discoverLinux lists a user-level desktop application: its desktop entry, the data folder named after its identifier,
// and for a user flatpak its install and app data. Nothing else is matched by name on Linux.
func (u *Uninstaller) discoverLinux(appPath string) Discovery {
	if clean, ok := cleanPath(appPath); ok && isUnderAny(clean, systemDesktopRoots) {
		return Discovery{ReasonCode: codeSystemPkg, Message: systemPackageMessage}
	}
	app, ref := u.checkApp(appPath, nil)
	if ref != nil {
		return Discovery{ReasonCode: ref.Code, Message: ref.Message}
	}
	running := u.isRunning(app.Path)
	verdict := u.linuxOwnership(app.BundleID, app.Path)
	flatpak := path.Dir(app.Path) == u.flatpakExportRoot()
	exact, notes := u.collectExact(app, u.exactSpecsLinux(app.BundleID, flatpak), verdict)
	return u.readyDiscovery(app, running, exact, nil, notes)
}

// linuxOwnership decides whether the desktop identifier names another entry too: a system entry, or the same name in
// another user folder. Such data is not selected by default, because the other entry may own it.
func (u *Uninstaller) linuxOwnership(id, appPath string) ownership {
	roots := append(append([]string{}, systemDesktopRoots...), u.userDesktopRoot(), u.flatpakExportRoot())
	for _, root := range roots {
		other := path.Join(root, id+".desktop")
		if other == appPath {
			continue
		}
		_, err := u.deps.Files.Lstat(other)
		switch {
		case err == nil:
			return ownership{Status: ownerShared, Owner: path.Base(root)}
		case !isMissing(err):
			return ownership{Status: ownerUnverified, Message: "The application index could not be checked"}
		}
	}
	return ownership{Status: ownerConfirmed}
}

// isUnderAny reports whether p is inside one of the roots.
func isUnderAny(p string, roots []string) bool {
	for _, root := range roots {
		if within(root, p) || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}
