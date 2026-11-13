package hyprland

import (
	"path/filepath"
	"regexp"
	"strings"
)

// includeMarker is the comment line setup writes above the include, so removal finds exactly its own lines.
const includeMarker = "Nevermind: shortcuts and window rules. Remove this line and the next to turn them off."

// moduleBase is the module's file name without its extension.
const moduleBase = "nevermind"

// IncludeLines are the two lines setup adds to the end of the entry file: the marker and the include.
func IncludeLines(f Format, modulePath string) []string {
	if f == FormatLua {
		// Hyprland resolves a relative module next to hyprland.lua; pcall keeps a missing module from stopping the rest.
		return []string{"-- " + includeMarker, `pcall(require, "` + moduleBase + `")`}
	}
	return []string{"# " + includeMarker, "source = " + modulePath}
}

// includePresent reports whether entry already loads the module: the lines setup adds, or an include the user wrote.
func includePresent(f Format, entry []byte, modulePath, home string) bool {
	for _, line := range strings.Split(string(entry), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "--") {
			continue
		}
		if f == FormatLua && luaRequiresModule.MatchString(line) {
			return true
		}
		if f == FormatConf {
			if path, ok := sourcedPath(line, home); ok && path == filepath.Clean(modulePath) {
				return true
			}
		}
	}
	return false
}

// luaRequiresModule matches require("nevermind") and pcall(require, "nevermind"), with either quote.
var luaRequiresModule = regexp.MustCompile(`require\s*[,(]?\s*\(?\s*["']` + moduleBase + `["']`)

func sourcedPath(line, home string) (string, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok || strings.TrimSpace(key) != "source" {
		return "", false
	}
	value, _, _ = strings.Cut(value, "#")
	value = strings.TrimSpace(value)
	if rest, ok := strings.CutPrefix(value, "~/"); ok {
		value = filepath.Join(home, rest)
	}
	return filepath.Clean(value), true
}

// withInclude appends the include lines to entry, after a newline when the file does not end with one.
func withInclude(entry []byte, lines []string) []byte {
	out := string(entry)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out + "\n" + strings.Join(lines, "\n") + "\n")
}

// withoutInclude removes exactly the include lines setup added (the marker and the line after it, when that line is
// the include), and nothing else. It reports whether it removed them.
func withoutInclude(entry []byte, lines []string) ([]byte, bool) {
	all := strings.Split(string(entry), "\n")
	for i := 0; i+1 < len(all); i++ {
		if strings.TrimSpace(all[i]) == lines[0] && strings.TrimSpace(all[i+1]) == lines[1] {
			start := i
			// Drop the blank line withInclude put before them.
			if start > 0 && strings.TrimSpace(all[start-1]) == "" {
				start--
			}
			return []byte(strings.Join(append(all[:start:start], all[i+2:]...), "\n")), true
		}
	}
	return entry, false
}
