package builder

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/sourcecheck"
)

// Installed is one extension the builder can see before it writes: its id and title, whether it is on, and the
// command and row titles its source declares.
type Installed struct {
	ID      string
	Title   string
	Enabled bool
	Titles  []string
}

// installedSection lists the installed extensions in the system prompt and tells the model not to write a duplicate.
func installedSection(installed []Installed) string {
	if len(installed) == 0 {
		return "## Installed extensions\n\nNone yet."
	}
	lines := []string{
		"## Installed extensions",
		"",
		"Check this list before you write an extension. If an installed extension already does what the user asks, do not write a duplicate of it: reply with one sentence that names its id and asks whether to change it, and no code blocks.",
		"",
	}
	for _, entry := range installed {
		state := "off"
		if entry.Enabled {
			state = "on"
		}
		lines = append(lines, fmt.Sprintf("- %s: %q (%s); rows: %s", entry.ID, entry.Title, state, strings.Join(quoteAll(entry.Titles), ", ")))
	}
	return strings.Join(lines, "\n")
}

func quoteAll(titles []string) []string {
	if len(titles) == 0 {
		return []string{"none"}
	}
	quoted := make([]string, len(titles))
	for i, title := range titles {
		quoted[i] = fmt.Sprintf("%q", title)
	}
	return quoted
}

// titleClash returns the installed extension, other than the candidate's own, that already declares a row title the
// candidate declares too, and that title. Two rows with one title is the duplicate the user would see.
func titleClash(candidate Candidate, installed []Installed) (Installed, string, bool) {
	own := sourcecheck.DisplayTitles(candidate.SourceFiles())
	for _, entry := range installed {
		if entry.ID == candidate.Manifest.ID {
			continue
		}
		for _, title := range own {
			for _, existing := range entry.Titles {
				if sourcecheck.SameTitle(title, existing) {
					return entry, title, true
				}
			}
		}
	}
	return Installed{}, "", false
}
