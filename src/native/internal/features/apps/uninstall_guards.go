package apps

import (
	"path"
	"strings"
)

// The guards decide which paths may ever be moved, and re-decide it right before each move. Every rule here is
// fail-closed: an item that cannot be proven to be inside an allowed root, not a link, owned by the user and
// unchanged since review is refused with a message the review can show.

// cleanPath returns the cleaned form of a path that uninstall was given, or false. The path must be absolute,
// must hold no ".." segment and no NUL byte. Cleaning removes doubled and trailing separators, so "/A.app/" and
// "/A.app" name the same item; case is kept, so a differently-cased spelling names a different item.
func cleanPath(p string) (string, bool) {
	if p == "" || !path.IsAbs(p) || strings.ContainsRune(p, 0) {
		return "", false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return "", false
		}
	}
	return path.Clean(p), true
}

// within reports whether target is strictly inside root. Both must be clean.
func within(root, target string) bool {
	return root != target && strings.HasPrefix(target, root+"/")
}

func refusal(code, message string) *Refusal {
	return &Refusal{Code: code, Message: message}
}

// lookupRefusal turns a failed Lstat into a refusal worded the way the review shows it.
func lookupRefusal(err error) *Refusal {
	if isMissing(err) {
		return refusal(codeMissing, "This item is no longer available")
	}
	return refusal(codeInaccessible, "This item cannot be accessed safely")
}

// inspectPath Lstat-walks every component of p from the root. A symbolic link anywhere on the way refuses the
// path, so a link can never redirect a move outside the folder that was checked.
func (u *Uninstaller) inspectPath(p string) (Entry, *Refusal) {
	var current string
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		current += "/" + part
		entry, err := u.deps.Files.Lstat(current)
		if err != nil {
			return Entry{}, lookupRefusal(err)
		}
		if entry.Symlink {
			return Entry{}, refusal(codeSymlink, "Refusing symbolic link: "+current)
		}
		if i == len(parts)-1 {
			return entry, nil
		}
	}
	return Entry{}, refusal(codeInvalidPath, "Selected item is not a valid path")
}

// inspectAllowed inspects the root and then p, and requires p to be strictly inside root. It returns p's entry.
func (u *Uninstaller) inspectAllowed(p, root string) (Entry, *Refusal) {
	rootEntry, ref := u.inspectPath(root)
	if ref != nil {
		return Entry{}, ref
	}
	if !rootEntry.Dir {
		return Entry{}, refusal(codeOutside, "Refusing path outside "+root)
	}
	entry, ref := u.inspectPath(p)
	if ref != nil {
		return Entry{}, ref
	}
	if !within(root, p) {
		return Entry{}, refusal(codeOutside, "Refusing path outside "+root)
	}
	return entry, nil
}

// checkedApp is an application that passed every rule, with what the review needs to bind to it.
type checkedApp struct {
	Path     string
	Identity Identity
	BundleID string
}

// checkApp applies the app rules: the location, the entry kind, that it is not Nevermind, a safe identifier, a
// writable parent, and (when expected is set) that it is still the app that was reviewed.
func (u *Uninstaller) checkApp(appPath string, expected *UninstallSnapshot) (checkedApp, *Refusal) {
	clean, ok := cleanPath(appPath)
	if !ok {
		return checkedApp{}, refusal(codeInvalidPath, "Selected item is not a valid path")
	}
	parent := path.Dir(clean)
	if !u.isAppFolder(parent) {
		return checkedApp{}, refusal("outside-allowlist", u.appLocationMessage())
	}
	entry, ref := u.inspectAllowed(clean, parent)
	if ref != nil {
		return checkedApp{}, ref
	}
	if !u.isAppEntry(clean, entry) {
		return checkedApp{}, refusal("invalid-app", "Selected item is not an application bundle")
	}
	bundleID, ref := u.readAppIdentifier(clean)
	if ref != nil {
		return checkedApp{}, ref
	}
	if u.isNevermind(clean, bundleID) {
		return checkedApp{}, refusal(codeSelf, "Nevermind cannot uninstall itself")
	}
	if !u.parentWritable(parent) {
		return checkedApp{}, refusal("app-parent-not-writable", "This app cannot be moved to Trash from its current folder")
	}
	checked := checkedApp{Path: clean, Identity: entry.Identity, BundleID: bundleID}
	if expected != nil && !matchesExpectedApp(checked, *expected) {
		return checkedApp{}, refusal(codeAppChanged, "The application changed after this uninstall view was opened")
	}
	return checked, nil
}

func matchesExpectedApp(checked checkedApp, expected UninstallSnapshot) bool {
	return sameIdentity(checked.Identity, expected.AppIdentity) &&
		checked.Path == expected.AppPath &&
		strings.EqualFold(checked.BundleID, expected.AppBundleID)
}

// isAppFolder reports whether parent is a folder apps may be selected from.
func (u *Uninstaller) isAppFolder(parent string) bool {
	if u.deps.Platform == "linux" {
		return parent == u.userDesktopRoot() || parent == u.flatpakExportRoot()
	}
	return parent == "/Applications" || parent == u.userApplicationsRoot()
}

func (u *Uninstaller) appLocationMessage() string {
	if u.deps.Platform == "linux" {
		return "Only applications installed for your user can be uninstalled here"
	}
	return "App must be in /Applications or ~/Applications"
}

// isAppEntry reports whether an entry is the kind of item an app is on this platform: a bundle folder on macOS,
// a desktop file on Linux.
func (u *Uninstaller) isAppEntry(p string, entry Entry) bool {
	if u.deps.Platform == "linux" {
		return !entry.Dir && strings.HasSuffix(p, ".desktop")
	}
	return entry.Dir && strings.HasSuffix(strings.ToLower(p), ".app")
}

// readAppIdentifier returns the app's identifier, validated. On Linux it is the desktop file's name without the
// extension; on macOS the plist's CFBundleIdentifier.
func (u *Uninstaller) readAppIdentifier(p string) (string, *Refusal) {
	raw := ""
	if u.deps.Platform == "linux" {
		raw = strings.TrimSuffix(path.Base(p), ".desktop")
	} else {
		if u.deps.ReadBundleID == nil {
			return "", refusal("plist", "This app’s metadata could not be read")
		}
		id, err := u.deps.ReadBundleID(p)
		if err != nil {
			return "", refusal("plist", "This app’s metadata could not be read")
		}
		raw = id
	}
	id, ok := ValidateBundleID(raw)
	if !ok {
		return "", refusal("bundle-id", "The app has no safe bundle identifier")
	}
	return id, nil
}

// isNevermind reports whether the app is this copy of Nevermind, by path or by identifier.
func (u *Uninstaller) isNevermind(p, bundleID string) bool {
	if u.deps.NevermindAppPath != "" && path.Clean(u.deps.NevermindAppPath) == p {
		return true
	}
	return u.deps.NevermindBundleID != "" && strings.EqualFold(bundleID, u.deps.NevermindBundleID)
}

// parentWritable checks effective access to the directory after verifying its identity and type.
func (u *Uninstaller) parentWritable(parent string) bool {
	entry, err := u.deps.Files.Lstat(parent)
	if err != nil || entry.Symlink || !entry.Dir {
		return false
	}
	return u.deps.DirectoryAccess != nil && u.deps.DirectoryAccess.CanWrite(parent)
}

// isRunning asks the platform whether the app runs. Any failure counts as running: a move is never attempted on
// a guess.
func (u *Uninstaller) isRunning(appPath string) bool {
	if u.deps.IsRunning == nil {
		return true
	}
	running, err := u.deps.IsRunning(appPath)
	return err != nil || running
}

// revalidate re-checks one candidate immediately before it is moved: the path still resolves inside its root with no
// link on the way, the identity is the one reviewed, and an associated item is still the user's.
func (u *Uninstaller) revalidate(c Candidate) *Refusal {
	clean, ok := cleanPath(c.Path)
	if !ok {
		return refusal(codeInvalidPath, "Selected item is not a valid path")
	}
	entry, ref := u.inspectAllowed(clean, c.Root)
	if ref != nil {
		return ref
	}
	if !sameIdentity(entry.Identity, c.Identity) {
		return refusal(codeChanged, "Item changed after confirmation")
	}
	if c.Kind == KindAssociated && entry.Identity.UID != u.deps.UID {
		return refusal(codeOwner, "Item is no longer owned by the current user")
	}
	return nil
}
