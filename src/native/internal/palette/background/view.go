// Package background is the Background Tasks page: every scheduled job with its status, its last run and its
// triggers, the run history of each job, and the actions to run it now, turn it on or off, and clear its error.
// The page reads a Source and changes jobs only through the Source's controls.
package background

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Source is what the page reads and drives. The scheduler satisfies it.
type Source interface {
	Snapshot() []jobs.Snapshot
	RunNow(id string) error
	SetEnabled(id string, enabled bool) error
	ClearError(id string) error
}

// View is the Background Tasks list. A nil source shows the empty state, as a scheduler that has not started does.
func View(source Source, now func() time.Time) *palette.View {
	return &palette.View{
		Title:       "Background Tasks",
		Placeholder: "Search background tasks",
		EmptyTitle:  "No background tasks registered.",
		Hint:        "↵ History",
		Search: func(query string) []search.Item {
			return listRows(source, now, query)
		},
	}
}

// Details is the run history of one job, newest first.
func Details(source Source, id string) *palette.View {
	return &palette.View{
		Title:       title(source, id),
		Placeholder: "Search runs",
		EmptyTitle:  "No runs recorded yet.",
		Search: func(query string) []search.Item {
			return historyRows(source, id, query)
		},
	}
}

func title(source Source, id string) string {
	if snap, ok := findJob(source, id); ok {
		return snap.Title
	}
	return "Background Task"
}

func listRows(source Source, now func() time.Time, query string) []search.Item {
	var items []search.Item
	for _, snap := range snapshotsOf(source) {
		item := listRow(source, snap, now)
		if matchesQuery(item.Title+" "+item.Subtitle, query) {
			items = append(items, item)
		}
	}
	return items
}

func historyRows(source Source, id string, query string) []search.Item {
	snap, ok := findJob(source, id)
	if !ok {
		return nil
	}
	var items []search.Item
	for _, entry := range snap.History {
		item := historyRow(entry)
		if matchesQuery(item.Title+" "+item.Subtitle, query) {
			items = append(items, item)
		}
	}
	return items
}

func snapshotsOf(source Source) []jobs.Snapshot {
	if source == nil {
		return nil
	}
	return source.Snapshot()
}

func findJob(source Source, id string) (jobs.Snapshot, bool) {
	for _, snap := range snapshotsOf(source) {
		if snap.ID == id {
			return snap, true
		}
	}
	return jobs.Snapshot{}, false
}

func matchesQuery(text, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	return needle == "" || strings.Contains(strings.ToLower(text), needle)
}
