// Package extitems shows the menu bar items extensions declare. It reads them from the extension host (a Source),
// normalizes them to the limits every host shares, and keeps one menubar.Item per extension item on the scheduler.
// A render that fails keeps the last good menu and marks it as not responding; a disabled extension's items close.
//
// Layer: service tier (menubar). It imports api, menubar, logx and platform.
package extitems

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Limits every host applies to what an extension declares. They match the limits the Electron host enforces, so an
// extension that works on one works on the other.
const (
	maxBarsPerExtension = 3
	maxSections         = 8
	maxRowsPerSection   = 40
	maxSubmenuDepth     = 2
	maxRowText          = 120
	maxBarTitle         = 24
	maxBarID            = 64
	mainBarID           = "main"
)

// Normalize returns the menu bar items a host shows for the descriptors an extension rendered, and the issues that
// made it drop or shorten something. Nothing is an error: a bad row costs that row, not the item.
func Normalize(bars []api.MenuBar, extensionTitle string) ([]api.MenuBar, []string) {
	var issues []string
	if len(bars) > maxBarsPerExtension {
		issues = append(issues, fmt.Sprintf("extensions show at most %d menu bar items", maxBarsPerExtension))
		bars = bars[:maxBarsPerExtension]
	}
	seen := map[string]bool{}
	normalized := make([]api.MenuBar, 0, len(bars))
	for index, bar := range bars {
		bar, id, ok := normalizeBar(bar, index, extensionTitle, &issues)
		if !ok || seen[id] {
			if ok {
				issues = append(issues, fmt.Sprintf("menu bar id %q is used twice", id))
			}
			continue
		}
		seen[id] = true
		normalized = append(normalized, bar)
	}
	return normalized, issues
}

// normalizeBar cleans one item. The index-zero item may omit its id; later items need one.
func normalizeBar(bar api.MenuBar, index int, extensionTitle string, issues *[]string) (api.MenuBar, string, bool) {
	id := boundedText(bar.ID, maxBarID)
	if id == "" {
		if index > 0 {
			*issues = append(*issues, "extra menu bar items need a stable id")
			return api.MenuBar{}, "", false
		}
		id = mainBarID
	}
	if bar.Popover != nil {
		*issues = append(*issues, "menu bar popovers are not shown yet; the menu is used instead")
	}
	return api.MenuBar{
		ID:       id,
		Icon:     normalizeIcon(bar.Icon, issues),
		Title:    truncateTitle(bar.Title, issues),
		Tooltip:  orDefault(boundedText(bar.Tooltip, maxRowText), boundedText(extensionTitle, maxRowText)),
		Sections: normalizeSections(bar.Sections, issues),
	}, id, true
}

// normalizeIcon keeps a curated glyph and falls back to the default one, with an issue for an unknown name.
func normalizeIcon(icon api.MenuBarIcon, issues *[]string) api.MenuBarIcon {
	if icon == "" {
		return api.MenuIconDefault
	}
	if api.KnownMenuBarIcon(icon) {
		return icon
	}
	*issues = append(*issues, fmt.Sprintf("unknown menu bar icon %q", icon))
	return api.MenuIconDefault
}

// truncateTitle shortens a title to the visible length, with an issue when it was longer.
func truncateTitle(title string, issues *[]string) string {
	cleaned := collapseSpace(title)
	if utf8.RuneCountInString(cleaned) <= maxBarTitle {
		return cleaned
	}
	*issues = append(*issues, fmt.Sprintf("menu bar titles show at most %d characters", maxBarTitle))
	return truncateRunes(cleaned, maxBarTitle)
}

func normalizeSections(sections []api.MenuSection, issues *[]string) []api.MenuSection {
	if len(sections) > maxSections {
		*issues = append(*issues, fmt.Sprintf("menu bars show at most %d sections", maxSections))
		sections = sections[:maxSections]
	}
	normalized := make([]api.MenuSection, 0, len(sections))
	for _, section := range sections {
		rows := normalizeRows(section.Rows, 1, issues)
		if len(rows) == 0 {
			continue
		}
		normalized = append(normalized, api.MenuSection{Title: boundedText(section.Title, maxRowText), Rows: rows})
	}
	return normalized
}

// normalizeRows keeps the rows that have an id and a title. depth counts from the section: a section's rows may
// carry one submenu, and the submenu's rows carry none.
func normalizeRows(rows []api.MenuRow, depth int, issues *[]string) []api.MenuRow {
	if len(rows) > maxRowsPerSection {
		*issues = append(*issues, fmt.Sprintf("menu sections show at most %d rows", maxRowsPerSection))
		rows = rows[:maxRowsPerSection]
	}
	normalized := make([]api.MenuRow, 0, len(rows))
	for _, row := range rows {
		if cleaned, ok := normalizeRow(row, depth, issues); ok {
			normalized = append(normalized, cleaned)
		}
	}
	return normalized
}

func normalizeRow(row api.MenuRow, depth int, issues *[]string) (api.MenuRow, bool) {
	id := boundedText(row.ID, maxRowText)
	title := boundedText(row.Title, maxRowText)
	if id == "" || title == "" {
		*issues = append(*issues, "menu rows need an id and a title")
		return api.MenuRow{}, false
	}
	cleaned := api.MenuRow{
		ID:       id,
		Title:    title,
		Subtitle: boundedText(row.Subtitle, maxRowText),
		Icon:     row.Icon,
		Checked:  row.Checked,
		Disabled: row.Disabled,
		Shortcut: row.Shortcut,
	}
	if len(row.Items) > 0 {
		cleaned.Items = submenuItems(row.Items, depth, issues)
		return cleaned, true
	}
	cleaned.Action = normalizeAction(row.Action, issues)
	return cleaned, true
}

// submenuItems keeps a submenu one level deep. Deeper nesting is refused with an issue.
func submenuItems(items []api.MenuRow, depth int, issues *[]string) []api.MenuRow {
	if depth >= maxSubmenuDepth {
		*issues = append(*issues, "submenus nest one level only")
		return nil
	}
	return normalizeRows(items, depth+1, issues)
}

// normalizeAction drops an action that does not validate, so its row shows disabled rather than failing the item.
func normalizeAction(action *api.Action, issues *[]string) *api.Action {
	if action == nil {
		return nil
	}
	if err := action.Validate(); err != nil {
		*issues = append(*issues, "menu row action is dropped: "+err.Error())
		return nil
	}
	return action
}

// boundedText collapses whitespace, trims, and shortens to max runes. Empty text stays empty.
func boundedText(text string, limit int) string {
	return truncateRunes(collapseSpace(text), limit)
}

func collapseSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit-1]) + "…"
}

func orDefault(text, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
