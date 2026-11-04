package windows

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// settingsView edits the gaps, sizes and behavior the layouts use. Every edit saves at once and the list stays
// where it is; a number is typed in its own view.
func settingsView(d Deps) *palette.View {
	return &palette.View{
		Title: "Window Management Settings", Placeholder: "Search Settings",
		Search: func(q string) []search.Item {
			return settingsRows(d, q)
		},
	}
}

func settingsRows(d Deps, q string) []search.Item {
	current := d.Settings.Current()
	rows := []search.Item{}
	for _, def := range settingDefs {
		if matchesQuery(def.title, q) {
			rows = append(rows, settingRow(d, def, current))
		}
	}
	if matchesQuery("Reset to Defaults", q) {
		rows = append(rows, resetRow(d))
	}
	return rows
}

func matchesQuery(text, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(text), strings.ToLower(q))
}

func settingRow(d Deps, def settingDef, current geometry.Settings) search.Item {
	return search.Item{
		Kind: search.KindWindows, Title: def.title, Section: def.section,
		Accessory: def.read(current), Icon: iconOf(def.kind),
		Actions: []search.Action{editAction(d, def)},
	}
}

func editAction(d Deps, def settingDef) search.Action {
	switch def.kind {
	case kindFlag:
		return search.Action{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
			saveSetting(d, h, def.toggle)
		}}
	case kindChoice:
		return search.Action{Title: "Next", Dismiss: search.DismissNever, Run: func(h search.Host) {
			saveSetting(d, h, def.cycle)
		}}
	}
	return search.Action{Title: "Change…", Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(numberView(d, def))
	}}
}

func iconOf(kind settingKind) string {
	switch kind {
	case kindFlag:
		return "toggle-right"
	case kindChoice:
		return "list"
	}
	return "ruler"
}

func resetRow(d Deps) search.Item {
	return search.Item{
		Kind: search.KindWindows, Title: "Reset to Defaults", Subtitle: "Restore every window management setting",
		Section: "Defaults", Icon: "rotate-ccw",
		Actions: []search.Action{{Title: "Reset", Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := d.Settings.Reset(); err != nil {
				h.Toast("Could not save the window settings: " + err.Error())
			}
			h.Refresh()
		}}},
	}
}

// saveSetting applies one edit to the settings, reports a failed save, and redraws the list.
func saveSetting(d Deps, h search.Host, edit func(*geometry.Settings)) {
	if err := d.Settings.Update(edit); err != nil {
		h.Toast("Could not save the window settings: " + err.Error())
	}
	h.Refresh()
}

// numberView asks for one whole number. The typed text is the query, so the input is the palette's own field.
func numberView(d Deps, def settingDef) *palette.View {
	return &palette.View{
		Title: def.title, Placeholder: placeholderOf(def),
		Search: func(q string) []search.Item {
			return numberRows(d, def, q)
		},
	}
}

func placeholderOf(def settingDef) string {
	if def.override != nil {
		return fmt.Sprintf("Blank follows the outer gap, or type %d to %d", def.min, def.max)
	}
	return fmt.Sprintf("Type a value from %d to %d%s", def.min, def.max, def.unit)
}

func numberRows(d Deps, def settingDef, q string) []search.Item {
	if q == "" {
		return []search.Item{emptyNumberRow(d, def)}
	}
	value, message := def.parse(q)
	if message != "" {
		return []search.Item{{Kind: search.KindWindows, Title: message, Icon: "triangle-alert"}}
	}
	return []search.Item{{
		Kind: search.KindWindows, Title: fmt.Sprintf("Set %s to %s%s", def.title, q, def.unit), Icon: "check",
		Actions: []search.Action{saveNumberAction(d, def, value)},
	}}
}

// emptyNumberRow shows the range while nothing is typed. An edge gap offers to clear the override instead.
func emptyNumberRow(d Deps, def settingDef) search.Item {
	if def.override != nil {
		return search.Item{Kind: search.KindWindows, Title: "Follow the outer gap", Icon: "ruler",
			Actions: []search.Action{saveNumberAction(d, def, nil)}}
	}
	return search.Item{Kind: search.KindWindows, Title: "Type a value", Subtitle: "Current: " + def.read(d.Settings.Current()), Icon: "pencil"}
}

func saveNumberAction(d Deps, def settingDef, value *float64) search.Action {
	return search.Action{Title: "Save", Dismiss: search.DismissNever, Run: func(h search.Host) {
		if err := d.Settings.Update(func(s *geometry.Settings) { def.assign(s, value) }); err != nil {
			h.Toast("Could not save the window settings: " + err.Error())
		}
		h.Pop()
	}}
}
