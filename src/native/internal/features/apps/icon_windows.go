package apps

import (
	"regexp"
	"strings"
)

// Windows shortcut icons. A Start Menu shortcut (.lnk) names a target executable and maybe an icon resource; the
// app's Visual Elements manifest may name a logo next to the executable. These functions order the sources to try.
// Paths follow Windows rules (backslash separators, case-insensitive); the reading of the .lnk and of the manifest
// is injected, so the ordering is tested on every OS.

// ShortcutLink is what reading a .lnk yields: the target and an optional icon resource.
type ShortcutLink struct {
	Target    string
	Icon      string
	IconIndex int
}

// IconSource is one place to read an icon from: a file, or a resource index of a file when Indexed is set.
type IconSource struct {
	Path     string
	Resource int
	Indexed  bool
}

var winSeparators = regexp.MustCompile(`[\\/]+`)

var visualElementsLogoPattern = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bSquare44x44Logo\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)\bSquare70x70Logo\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)\bSquare150x150Logo\s*=\s*["']([^"']+)["']`),
}

// ShortcutIconSources returns where to read the icon of the shortcut at appPath, most specific first: the
// shortcut's icon resource, the Visual Elements logo confined to the application folder, the target, then the
// shortcut itself. An unreadable shortcut yields only the shortcut.
func ShortcutIconSources(appPath string, readShortcut func(string) (ShortcutLink, error), readText func(string) (string, error)) []IconSource {
	shortcut, err := readShortcut(appPath)
	if err != nil {
		return []IconSource{{Path: appPath}}
	}
	executable := winBaseWithoutExt(shortcut.Target)
	manifestPath := winJoin(winDir(shortcut.Target), executable+".VisualElementsManifest.xml")
	manifest, err := readText(manifestPath)
	if err != nil {
		manifest = ""
	}
	var sources []IconSource
	if shortcut.Icon != "" {
		sources = append(sources, IconSource{Path: shortcut.Icon, Resource: shortcut.IconIndex, Indexed: true})
	}
	logo := visualElementsLogoPath(shortcut.Target, manifest)
	for _, candidate := range []string{logo, shortcut.Target, appPath} {
		if candidate == "" || containsPlainSource(sources, candidate) {
			continue
		}
		sources = append(sources, IconSource{Path: candidate})
	}
	return sources
}

func containsPlainSource(sources []IconSource, candidate string) bool {
	for _, source := range sources {
		if !source.Indexed && strings.EqualFold(source.Path, candidate) {
			return true
		}
	}
	return false
}

// visualElementsLogoPath returns the first logo the manifest names, resolved against the target's folder. A logo
// that resolves outside that folder is refused and yields "".
func visualElementsLogoPath(target, manifest string) string {
	logo := ""
	for _, pattern := range visualElementsLogoPattern {
		if m := pattern.FindStringSubmatch(manifest); m != nil {
			logo = m[1]
			break
		}
	}
	if logo == "" {
		return ""
	}
	appDirectory := winDir(target)
	resolved := winResolve(appDirectory, logo)
	if !winWithin(appDirectory, resolved) {
		return ""
	}
	return resolved
}

// winDir returns the folder part of a Windows path.
func winDir(p string) string {
	i := strings.LastIndexAny(p, `\/`)
	if i < 0 {
		return "."
	}
	return p[:i]
}

// winBaseWithoutExt returns the file name of a Windows path without its extension.
func winBaseWithoutExt(p string) string {
	base := p[strings.LastIndexAny(p, `\/`)+1:]
	if dot := strings.LastIndex(base, "."); dot > 0 {
		return base[:dot]
	}
	return base
}

func winJoin(dir, name string) string {
	return dir + `\` + name
}

// winResolve joins rel onto dir and collapses "." and ".." segments, the way path.win32.resolve does for a relative
// path. An absolute rel is returned cleaned as it is.
func winResolve(dir, rel string) string {
	if isWinAbsolute(rel) {
		return winClean(rel)
	}
	return winClean(dir + `\` + rel)
}

func isWinAbsolute(p string) bool {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return strings.HasPrefix(p, `\\`)
}

// winClean collapses separators and "." and ".." segments. A ".." above the root stays, so a result that escapes
// its base is still visible to winWithin.
func winClean(p string) string {
	root := ""
	if len(p) >= 2 && p[1] == ':' {
		root = p[:2]
		p = p[2:]
	}
	var out []string
	for _, segment := range winSeparators.Split(p, -1) {
		switch segment {
		case "", ".":
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else {
				out = append(out, "..")
			}
		default:
			out = append(out, segment)
		}
	}
	return root + `\` + strings.Join(out, `\`)
}

// winWithin reports whether target lies inside base (case-insensitively), not counting base itself.
func winWithin(base, target string) bool {
	b := strings.ToLower(winClean(base))
	t := strings.ToLower(winClean(target))
	return strings.HasPrefix(t, b+`\`)
}
