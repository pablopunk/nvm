package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// scoreExtensionFailure ranks a failed extension's pointer row below every real root row, the web search fallback
// included, so it is never the top hit.
const scoreExtensionFailure = 1

// failureRows is the pointer row a failed extension may show at the root. It appears only when the query names the
// extension, and its action opens the Extensions manager, where the reason and the runtime state live.
func failureRows(p *App, extID, title, query string, failure *api.Failure) []search.Item {
	if !namesExtension(query, extID, title) {
		return nil
	}
	return []search.Item{{
		Kind: title, Title: title + " is not responding", Subtitle: failure.Message, Icon: "triangle-alert",
		Tone: "red", Accessory: "Failed", Score: scoreExtensionFailure,
		Actions: []search.Action{{Title: "Show in Extensions", Run: func(h search.Host) { h.Push(p.extensionsView()) }}},
	}}
}

// namesExtension is true when the query is part of the extension's id or title, ignoring case.
func namesExtension(query, extID, title string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	return q != "" && (strings.Contains(strings.ToLower(extID), q) || strings.Contains(strings.ToLower(title), q))
}
