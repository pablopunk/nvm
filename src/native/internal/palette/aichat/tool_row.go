package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// Copy the transcript shows for a tool call that is still waiting for the user.
const copyAwaiting = "Waiting for you. Enter allows it, ⌘D refuses it."

// toolRow draws one tool call of the transcript, the same for every chat: its icon and title, its short detail, the
// line that says what it is doing or how it ended, and, once it has ended, a Result disclosure that opens what the
// model read. A call waiting for the user says which keys answer it. The transcript never shows what a call was given
// beyond its short detail, so a written extension's source stays out of the chat.
func toolRow(c *ui.Context, p *page, index int, m chat.Message) {
	k := design.Tokens(c)
	run := m.Tool
	ui.Column(c).Gap(4).Padding(8, 12).Radius(8).Background(k.Surface).Border(1, k.EdgeSubtle).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, design.Icon(toolIcon(run.Name))).FontSize(12).TextColor(k.Muted)
			ui.Text(c, run.Label).FontSize(12).FontWeight(600).TextColor(k.Strong).NoWrap()
			if run.Input != "" {
				ui.Text(c, run.Input).FontSize(12).TextColor(k.Muted).MaxLines(1)
			}
		})
		ui.Text(c, toolLine(m)).FontSize(11).TextColor(toolLineColor(k, m.Status)).MaxLines(2)
		if resultReady(m) {
			resultDisclosure(c, p, index, m.Text)
		}
	})
}

// resultReady reports whether a tool row has a result to show: the call has ended and the model read something.
func resultReady(m chat.Message) bool {
	return m.Text != "" && m.Status != chat.StatusRunning && m.Status != chat.StatusAwaiting
}

// resultDisclosure is the closed Result disclosure of a tool row; opened, it shows the newest part of the result.
func resultDisclosure(c *ui.Context, p *page, index int, result string) {
	k := design.Tokens(c)
	open := p.resultOpen[index]
	if ui.Collapsible(c, "Result", &open, func() {
		ui.Text(c, clipResult(result)).FontSize(11).TextColor(k.Muted)
	}).Changed() {
		p.resultOpen[index] = open
	}
}

// clipResult is the start of a result, at most shownResultBytes, cut on a whole rune and marked when cut.
func clipResult(result string) string {
	if len(result) <= shownResultBytes {
		return result
	}
	return truncateBytesForShow(result, shownResultBytes) + "…"
}

// toolIcon names the icon of a tool. A tool without one of its own shows the sparkles.
func toolIcon(name string) string {
	icons := map[string]string{
		"search_files":   "search",
		"read_file":      "file-text",
		"list_folder":    "folder",
		"list_apps":      "app-window",
		"open_app":       "play",
		"open_url":       "globe",
		"read_clipboard": "clipboard",
		"copy_text":      "copy",
		"trash_file":     "trash-2",
	}
	if icon, ok := icons[name]; ok {
		return icon
	}
	return "sparkles"
}

// toolLine is the second line of a tool row: what the call is doing, or the outcome it ended with.
func toolLine(m chat.Message) string {
	switch m.Status {
	case chat.StatusRunning:
		return "Running…"
	case chat.StatusAwaiting:
		return copyAwaiting
	case chat.StatusCancelled:
		return "Stopped before it finished."
	}
	if m.Tool != nil && m.Tool.Outcome != "" {
		return m.Tool.Outcome
	}
	return "Done."
}

// toolLineColor colors the outcome line: a failure reads as a failure, a wait is accented, anything else is muted.
func toolLineColor(k *design.TokenSet, status chat.Status) ui.Color {
	switch status {
	case chat.StatusFailed:
		return k.Danger
	case chat.StatusAwaiting:
		return k.Accent
	}
	return k.Muted
}
