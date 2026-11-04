// Package diagnostics is the palette page for recent errors: each record with a copyable support reference and its safe
// diagnostics, and how the record is reported. The page reads the recorder through Records and never writes to it.
package diagnostics

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Records is what the page reads: the recent records, newest first, and the export text of one of them. The recorder
// satisfies it.
type Records interface {
	Recent() []diag.Record
	Export(ref string) string
}

// Copy puts text on the clipboard for a row's action. The composition root supplies it, so the page holds no port.
type Copy func(h search.Host, text string)

// Reporting returns the reporting state now. It is a function so the page follows a change made while it is open.
type Reporting func() analytics.Reporting

// View is the Recent Errors page.
func View(records Records, reporting Reporting, copyText Copy) *palette.View {
	return &palette.View{
		Title:       "Recent Errors",
		Placeholder: "Search recent errors",
		Hint:        "Technical details only",
		EmptyTitle:  "No recent errors recorded",
		Search: func(q string) []search.Item {
			return filterRows(rows(records, reporting(), copyText), q)
		},
	}
}

// ReportingLabel says how a record is reported. Remote availability is never confirmed here: a sent event can still
// be lost on the way.
func ReportingLabel(state analytics.Reporting) string {
	switch state {
	case analytics.Off:
		return "Not reported"
	case analytics.Requested:
		return "Reporting requested · remote availability unknown"
	default:
		return "Stored on this computer"
	}
}

// rows is one row per record, with the reference and the safe diagnostics as its actions.
func rows(records Records, state analytics.Reporting, copyText Copy) []search.Item {
	recent := records.Recent()
	items := make([]search.Item, 0, len(recent))
	for _, record := range recent {
		items = append(items, row(records, record, state, copyText))
	}
	return items
}

// row shows a problem report as one, and any other record under its component.
func row(records Records, record diag.Record, state analytics.Reporting, copyText Copy) search.Item {
	ref := record.Ref
	title, icon := record.Component, "circle-alert"
	if record.Component == analytics.ProblemComponent {
		title, icon = "Problem report", "message-circle"
	}
	return search.Item{
		ID:        ref,
		Title:     title,
		Subtitle:  ref + " · " + record.Time.UTC().Format("2006-01-02 15:04:05"),
		Icon:      icon,
		Accessory: ReportingLabel(state),
		Actions: []search.Action{
			{Title: "Copy Error Reference", Run: func(h search.Host) { copyText(h, ref) }},
			{Title: "Copy Safe Diagnostics", Run: func(h search.Host) { copyText(h, records.Export(ref)) }},
		},
	}
}

// filterRows keeps the rows whose title, subtitle or reference contains q, ignoring case.
func filterRows(items []search.Item, q string) []search.Item {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	return slices.DeleteFunc(items, func(item search.Item) bool {
		return !strings.Contains(strings.ToLower(item.Title+" "+item.Subtitle), q)
	})
}
