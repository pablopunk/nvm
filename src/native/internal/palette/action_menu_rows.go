package palette

import (
	"sort"
	"strings"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actionMenuRows maps visible menu rows to their filtered actions; headers never take a choice.
func actionMenuRows(acts []search.Action, chosen int, query string) (rows []design.MenuRow, rowAction []int) {
	defaultIndex := defaultMenuIndex(acts)
	group := ""
	for i, a := range acts {
		if strings.TrimSpace(query) == "" && a.Section != "" && a.Section != group {
			rows = append(rows, design.MenuRow{Title: a.Section, Header: true})
			rowAction = append(rowAction, -1)
		}
		group = a.Section
		subtitle := a.Subtitle
		if a.Disabled != "" {
			subtitle = a.Disabled
		}
		rows = append(rows, design.MenuRow{Title: a.Title, Subtitle: subtitle, Key: KeyLabel(a.Label), Enter: a.Label == "" && i == defaultIndex, Selected: i == chosen, Disabled: a.Disabled != ""})
		rowAction = append(rowAction, i)
	}
	return
}

// rankedMenuActions follows 1.x rankActionRows: strongest title/subtitle score, title tie-break, stable source order.
func rankedMenuActions(acts []search.Action, query string) []search.Action {
	type match struct {
		action       search.Action
		score, title int
	}
	matches := make([]match, 0, len(acts))
	q := strings.TrimSpace(query)
	for _, a := range acts {
		if a.MenuHidden {
			continue
		}
		title := search.ScoreText(a.Title, q)
		subtitle := a.Subtitle
		if a.Disabled != "" {
			subtitle = a.Disabled
		}
		score := max(title, search.ScoreText(subtitle, q))
		if q == "" || score > 0 {
			matches = append(matches, match{a, score, title})
		}
	}
	if q != "" {
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].score != matches[j].score {
				return matches[i].score > matches[j].score
			}
			return matches[i].title > matches[j].title
		})
	}
	out := make([]search.Action, len(matches))
	for i, m := range matches {
		out[i] = m.action
	}
	return out
}

func defaultMenuIndex(acts []search.Action) int {
	for i, a := range acts {
		if !a.MenuOnly {
			return i
		}
	}
	return -1
}
