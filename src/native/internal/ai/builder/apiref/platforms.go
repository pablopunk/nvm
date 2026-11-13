package apiref

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// writePlatforms lists the operations that exist on some operating systems only, from api.PlatformOperations. An
// extension whose source calls one is limited to those systems without any manifest field: Nevermind does not run it
// elsewhere, and it stays installed and enabled for the system it is for. That is information for the author, not an
// error, and the author needs no OS check around the call.
func (index *pkgIndex) writePlatforms(b *strings.Builder) {
	if index.terse {
		index.writePlatformsTerse(b)
		return
	}
	b.WriteString("## Platform availability\n\n")
	b.WriteString("These operations exist on some operating systems only. An extension that calls one works on those systems only: " +
		"Nevermind derives that from the source, runs the extension nowhere else, and keeps it installed and enabled for the system it is for. " +
		"It is not an error, and no manifest field or OS check is needed. Every operation not listed works on every system.\n\n")
	for _, operation := range api.PlatformOperations {
		fmt.Fprintf(b, "- %s: %s. %s\n", operation.Name(), operation.Platforms.Label(), operation.Note)
	}
	b.WriteString("\n")
}

// writePlatformsTerse is the preloaded form: one line per set of OSes, the operation names with the shared Desktop()
// prefix dropped (the header names it), since the preloaded reference counts against every turn's request budget.
func (index *pkgIndex) writePlatformsTerse(b *strings.Builder) {
	var labels []string
	byLabel := map[string][]string{}
	for _, operation := range api.PlatformOperations {
		label := operation.Platforms.Label()
		if _, seen := byLabel[label]; !seen {
			labels = append(labels, label)
		}
		byLabel[label] = append(byLabel[label], strings.TrimPrefix(operation.Name(), "Desktop()."))
	}
	b.WriteString("## Platform-limited (Desktop())\n")
	for _, label := range labels {
		fmt.Fprintf(b, "%s: %s\n", label, strings.Join(byLabel[label], ", "))
	}
}
