package apps

import (
	"path"
	"strconv"
	"strings"
)

// Reasons shown in the review, as in 1.x.
const (
	reasonApplication = "Application bundle"
	reasonRunningApp  = "Running. It will be quit gracefully before anything is moved to Trash."
	reasonExactData   = "Exact bundle ID data"
	reasonUnverified  = "Other installed apps could not be ruled out; left unchecked"
	reasonPrefixMatch = "Bundle ID prefix match; may belong to another app"
	reasonNameMatch   = "Name match; may belong to another app"
)

// Ownership verdicts: whether the bundle identifier belongs to this app alone.
const (
	ownerConfirmed  = "confirmed"
	ownerShared     = "shared"
	ownerUnverified = "unverified"
)

// ownership is the verdict on an app's identifier: confirmed (no other app has it), shared with a named app, or
// unverified (another app could not be read, so nothing is selected by default).
type ownership struct {
	Status  string
	Owner   string
	Message string
}

// ownerRoots are the folders whose apps are compared against an identifier on macOS.
func (u *Uninstaller) ownerRoots() []string {
	return []string{"/Applications", u.userApplicationsRoot()}
}

// verifyOwnershipDarwin decides whether another app has the identifier. With the Spotlight port it asks Spotlight;
// without it, it reads the identifier of every app in the application folders.
func (u *Uninstaller) verifyOwnershipDarwin(bundleID, appPath string) ownership {
	if u.deps.FindApps != nil {
		return u.ownershipFromSpotlight(bundleID, appPath)
	}
	var appPaths []string
	for _, root := range u.ownerRoots() {
		names, err := u.deps.Files.ReadDir(root)
		if err != nil {
			if isMissing(err) {
				continue
			}
			return ownership{Status: ownerUnverified, Message: "The applications folders could not be fully read"}
		}
		for _, entry := range names {
			if strings.HasSuffix(entry.Name, ".app") {
				appPaths = append(appPaths, path.Join(root, entry.Name))
			}
		}
	}
	for _, other := range appPaths {
		if other == appPath {
			continue
		}
		id, err := u.deps.ReadBundleID(other)
		if err != nil || id == "" {
			return ownership{Status: ownerUnverified, Message: "Another installed app could not be read"}
		}
		if BundleIDsOverlap(id, bundleID) {
			return ownership{Status: ownerShared, Owner: appNameOf(other)}
		}
	}
	return ownership{Status: ownerConfirmed}
}

func (u *Uninstaller) ownershipFromSpotlight(bundleID, appPath string) ownership {
	paths, err := u.deps.FindApps(bundleID)
	if err != nil {
		return ownership{Status: ownerUnverified, Message: "The application index could not be checked"}
	}
	found := false
	var other string
	for _, p := range paths {
		clean, ok := cleanPath(p)
		if !ok {
			continue
		}
		if clean == appPath {
			found = true
		} else if other == "" {
			other = clean
		}
	}
	if !found {
		return ownership{Status: ownerUnverified, Message: "The application index is incomplete"}
	}
	if other != "" {
		return ownership{Status: ownerShared, Owner: appNameOf(other)}
	}
	return ownership{Status: ownerConfirmed}
}

func appNameOf(p string) string {
	return strings.TrimSuffix(path.Base(p), ".app")
}

func exactReason(v ownership) string {
	switch v.Status {
	case ownerConfirmed:
		return reasonExactData
	case ownerShared:
		return "May be shared with " + v.Owner + "; left unchecked"
	default:
		return reasonUnverified
	}
}

// discoverDarwin lists a macOS app's items: the bundle itself, the exact bundle-identifier data, and (unchecked)
// prefix and name matches.
func (u *Uninstaller) discoverDarwin(appPath string) Discovery {
	app, ref := u.checkApp(appPath, nil)
	if ref != nil {
		return Discovery{ReasonCode: ref.Code, Message: ref.Message}
	}
	running := u.isRunning(app.Path)
	verdict := u.verifyOwnershipDarwin(app.BundleID, app.Path)
	exact, notes := u.collectExact(app, u.exactSpecsDarwin(app.BundleID), verdict)
	known := []string{app.Path}
	for _, c := range exact {
		known = append(known, c.Path)
	}
	patterns, patternNotes := u.collectPatterns(app, known)
	return u.readyDiscovery(app, running, exact, patterns, append(notes, patternNotes...))
}

// readyDiscovery assembles the snapshot: the app first, then the exact data, then the pattern matches.
func (u *Uninstaller) readyDiscovery(app checkedApp, running bool, exact, patterns []Candidate, notes []Note) Discovery {
	appCandidate := Candidate{
		ID:                u.nextID(),
		Path:              app.Path,
		Kind:              KindApp,
		Slot:              "app",
		Root:              path.Dir(app.Path),
		Identity:          app.Identity,
		SelectedByDefault: true,
		Reason:            reasonApplication,
	}
	if running {
		appCandidate.Reason = reasonRunningApp
	}
	candidates := append([]Candidate{appCandidate}, exact...)
	candidates = append(candidates, patterns...)
	if len(candidates) > MaxCandidates {
		candidates = candidates[:MaxCandidates]
		notes = append(notes, Note{Code: "candidate-limit", Message: "Only the first items are listed."})
	}
	return Discovery{
		Ready:      true,
		AppRunning: running,
		Notes:      notes,
		Snapshot: UninstallSnapshot{
			AppPath:     app.Path,
			AppBundleID: app.BundleID,
			AppIdentity: app.Identity,
			Candidates:  candidates,
		},
	}
}

// collectExact inspects each exact location. A missing location is counted in one note; a refused one is noted with
// its reason; a folder with more than MaxItems entries is refused so the review never lists what it cannot check.
func (u *Uninstaller) collectExact(app checkedApp, specs []locationSpec, verdict ownership) ([]Candidate, []Note) {
	var out []Candidate
	var notes []Note
	missing := 0
	seen := map[string]bool{}
	for _, spec := range specs {
		entry, ref := u.inspectAllowed(spec.Path, spec.Root)
		if ref != nil {
			if ref.Code == codeMissing {
				missing++
			} else {
				notes = append(notes, Note{Code: spec.Slot + "-" + ref.Code, Message: spec.Slot + ": " + ref.Message})
			}
			continue
		}
		if entry.Identity.UID != u.deps.UID {
			notes = append(notes, Note{Code: spec.Slot + "-owner", Message: spec.Slot + ": item is not owned by the current user"})
			continue
		}
		if seen[spec.Path] {
			continue
		}
		if entry.Dir {
			if _, err := u.deps.Files.Tree(spec.Path, MaxItems); err != nil {
				notes = append(notes, Note{Code: spec.Slot + "-" + codeTooLarge, Message: spec.Slot + ": more than 10,000 items; review it in Finder"})
				continue
			}
		}
		seen[spec.Path] = true
		out = append(out, Candidate{
			ID:                u.nextID(),
			Path:              spec.Path,
			Kind:              KindAssociated,
			Slot:              spec.Slot,
			Root:              spec.Root,
			Identity:          entry.Identity,
			SelectedByDefault: verdict.Status == ownerConfirmed,
			Reason:            exactReason(verdict),
		})
	}
	if missing > 0 {
		notes = append([]Note{missingLocationsNote(missing)}, notes...)
	}
	return out, notes
}

func missingLocationsNote(missing int) Note {
	noun := "locations were"
	if missing == 1 {
		noun = "location was"
	}
	return Note{Code: "missing-associated", Message: strconv.Itoa(missing) + " conventional associated " + noun + " not present."}
}

// collectPatterns scans the macOS folders for names that start with the identifier or equal the app's name. The
// matches are unchecked suggestions: never selected by default, at most MaxPatternMatches, each still inspected.
func (u *Uninstaller) collectPatterns(app checkedApp, known []string) ([]Candidate, []Note) {
	appName := appNameOf(app.Path)
	type match struct {
		spec locationSpec
		kind patternMatch
	}
	var matches []match
	inaccessible := false
	for _, root := range u.patternRootsDarwin() {
		entries, err := u.deps.Files.ReadDir(root.Root)
		if err != nil {
			if !isMissing(err) {
				inaccessible = true
			}
			continue
		}
		for _, entry := range entries {
			kind, ok := matchFolderName(entry.Name, app.BundleID, appName, root.AllowsNameMatch)
			if !ok {
				continue
			}
			p := path.Join(root.Root, entry.Name)
			if containsString(known, p) {
				continue
			}
			matches = append(matches, match{spec: locationSpec{Slot: root.Slot, Path: p, Root: root.Root, Kind: KindAssociated}, kind: kind})
		}
	}
	var notes []Note
	if len(matches) > MaxPatternMatches {
		notes = append(notes, Note{Code: "pattern-limit", Message: "Only the first " + strconv.Itoa(MaxPatternMatches) + " possible matches are listed."})
		matches = matches[:MaxPatternMatches]
	}
	if inaccessible {
		notes = append(notes, Note{Code: "incomplete-pattern-scan", Message: "Some locations could not be scanned; related files may be missing."})
	}
	var out []Candidate
	for _, m := range matches {
		entry, ref := u.inspectAllowed(m.spec.Path, m.spec.Root)
		if ref != nil || entry.Identity.UID != u.deps.UID {
			continue
		}
		reason := reasonNameMatch
		if m.kind == matchPrefix {
			reason = reasonPrefixMatch
		}
		out = append(out, Candidate{
			ID:       u.nextID(),
			Path:     m.spec.Path,
			Kind:     KindAssociated,
			Slot:     m.spec.Slot,
			Root:     m.spec.Root,
			Identity: entry.Identity,
			Reason:   reason,
		})
	}
	return out, notes
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
