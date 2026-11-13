package apps

import (
	"path"
	"regexp"
	"strings"
)

// DesktopEntry is what a Linux application needs from its .desktop file to find its process: the name shown, the
// command it runs, and the window class it reports.
type DesktopEntry struct {
	Name    string
	Command string
	WMClass string
}

// commLength is the longest process name the kernel reports (the comm field is 15 bytes plus a terminator).
const commLength = 15

var (
	hiddenEntryPattern   = regexp.MustCompile(`(?im)^(NoDisplay|Hidden)=true$`)
	entryNamePattern     = regexp.MustCompile(`(?m)^Name=(.+)$`)
	entryExecPattern     = regexp.MustCompile(`(?m)^Exec=(.+)$`)
	entryWMClassPattern  = regexp.MustCompile(`(?m)^StartupWMClass=(.+)$`)
	execFieldCodePattern = regexp.MustCompile(`\s*%[fFuUdDnNickvm]`)
	shellWordPattern     = regexp.MustCompile(`"[^"]+"|'[^']+'|\S+`)
	envAssignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// ParseDesktopEntry reads a .desktop file. Hidden and NoDisplay entries, and entries without a name or an Exec
// line, are not applications and return ok=false.
func ParseDesktopEntry(body string) (DesktopEntry, bool) {
	if hiddenEntryPattern.MatchString(body) {
		return DesktopEntry{}, false
	}
	name := firstGroup(entryNamePattern, body)
	exec := firstGroup(entryExecPattern, body)
	if name == "" || exec == "" {
		return DesktopEntry{}, false
	}
	return DesktopEntry{
		Name:    name,
		Command: strings.TrimSpace(execFieldCodePattern.ReplaceAllString(exec, "")),
		WMClass: firstGroup(entryWMClassPattern, body),
	}, true
}

func firstGroup(pattern *regexp.Regexp, text string) string {
	m := pattern.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

// ExecutableName returns the lowercased base name of the program a command runs, skipping leading "env" and
// NAME=value words, as the 1.x executableNameForLinuxCommand does.
func ExecutableName(command string) string {
	words := shellWords(strings.TrimSpace(command))
	for len(words) > 0 && (envAssignmentPattern.MatchString(words[0]) || path.Base(words[0]) == "env") {
		words = words[1:]
	}
	if len(words) == 0 {
		return ""
	}
	return strings.ToLower(path.Base(words[0]))
}

func shellWords(command string) []string {
	raw := shellWordPattern.FindAllString(command, -1)
	words := make([]string, 0, len(raw))
	for _, word := range raw {
		words = append(words, strings.Trim(word, `"'`))
	}
	return words
}

// MatchesProcessNames reports whether the entry's program or window class is one of the running process names.
// Names are compared lowercased; a name longer than the kernel's comm field matches its first 15 bytes, because
// that is what the process table reports.
func (e DesktopEntry) MatchesProcessNames(names map[string]bool) bool {
	for _, candidate := range []string{ExecutableName(e.Command), strings.ToLower(e.WMClass)} {
		if candidate == "" {
			continue
		}
		if names[candidate] {
			return true
		}
		if len(candidate) > commLength && names[candidate[:commLength]] {
			return true
		}
	}
	return false
}
