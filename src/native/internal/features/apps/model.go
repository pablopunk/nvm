package apps

import (
	"sort"
	"strings"
)

// App is an installed application as the index lists it. FirstSeenAt is the Unix millisecond the app first
// appeared, or 0 for apps that were there at the baseline.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	FirstSeenAt int64  `json:"firstSeenAt"`
}

// ScanResult is one listing of the installed apps. Complete is false when some location could not be read, so
// the listing must not be taken as the full baseline.
type ScanResult struct {
	Apps     []App
	Complete bool
}

// Scanner lists the installed apps. It may block; the index calls it off the UI thread.
type Scanner func() (ScanResult, error)

// normalizeName is the key apps are deduped by: the trimmed, lowercased display name.
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// DedupeAndSort keeps the last app listed for each normalized name, then sorts by name, case-insensitively.
// Two installs with the same name collapse into one row, as in 1.x; uninstall acts on the row's path.
func DedupeAndSort(apps []App) []App {
	byName := make(map[string]App, len(apps))
	order := make([]string, 0, len(apps))
	for _, app := range apps {
		key := normalizeName(app.Name)
		if _, seen := byName[key]; !seen {
			order = append(order, key)
		}
		byName[key] = app
	}
	out := make([]App, 0, len(order))
	for _, key := range order {
		out = append(out, byName[key])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := normalizeName(out[i].Name), normalizeName(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].Name < out[j].Name
	})
	return out
}
