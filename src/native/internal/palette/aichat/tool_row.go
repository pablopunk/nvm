package aichat

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"strings"
	"time"
)

func awaitingCopy() string {
	return "Waiting for you. Enter allows it, " + palette.KeyLabel("⌃D") + " refuses it."
}

// activity has one stable home above the composer. Completed tool protocol entries never become conversation rows.
func (x *ExtensionChat) activity(c *ui.Context) {
	if x.presentation.activity < 0 {
		return
	}
	m := x.spec.Messages[x.presentation.activity]
	k := design.Tokens(c)
	label := "Thinking"
	if m.Tool != nil {
		label = humanToolLabel(m.Tool.Label, m.Tool.Name)
	}
	awaiting := m.Status == chat.StatusAwaiting
	box := ui.Column(c).Key("chat-live-activity").Gap(4).Padding(4, 4, 8, 4)
	if !c.Preferences().ReduceMotion {
		box.Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, Position: true, Enter: &ui.Motion{Y: 2}, Exit: &ui.Motion{Y: -2}})
	}
	box.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if !awaiting && !c.Preferences().ReduceMotion {
				ui.Spinner(c)
			} else {
				icon := "sparkles"
				if awaiting {
					icon = "shield-check"
				}
				ui.Icon(c, design.Icon(icon)).FontSize(12).TextColor(k.Muted)
			}
			text := ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(k.Muted).SingleLine()
			if !c.Preferences().ReduceMotion {
				text.ContentTransition(ui.ContentTransition{Animation: &ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut}})
			}
		})
		if awaiting {
			if m.Tool.Outcome != "" {
				ui.Text(c, m.Tool.Outcome).FontSize(12).TextColor(k.Strong).MaxLines(3)
			}
			if m.Tool.Input != "" {
				ui.Text(c, m.Tool.Input).FontSize(11).TextColor(k.Muted).MaxLines(2)
			}
			ui.Text(c, awaitingCopy()).FontSize(11).TextColor(k.Muted)
		}
	})
}

var toolActivityLabels = map[string]string{
	"web_search": "Searching the web", "code_search": "Searching code", "fetch_content": "Reading a page",
	"get_search_content": "Reading a search result", "read_extension_api": "Reading the extension API",
	"list_capabilities": "Listing capabilities", "list_shortcuts": "Listing shortcuts", "read_app_logs": "Reading app logs",
	"list_extensions": "Listing extensions", "read_extension": "Reading an extension", "read_current_extension": "Reading the current extension",
	"write_extension": "Writing the extension", "read_preview_run": "Checking the preview run", "remove_extension": "Removing the extension",
	"validate_extension": "Validating the extension", "install_extension": "Installing the extension",
	"read_file": "Reading a file", "search_files": "Searching files", "list_folder": "Listing files", "list_apps": "Finding apps",
	"read_clipboard": "Reading the clipboard", "open_url": "Opening a link", "open_app": "Opening an app",
	"copy_text": "Copying text", "trash_file": "Moving a file to Trash", "run_program": "Running a program",
}

func humanToolLabel(label, name string) string {
	if label != "" && label != name && !strings.Contains(label, "_") {
		return label
	}
	if activity, ok := toolActivityLabels[name]; ok {
		return activity
	}
	if name == "" {
		name = label
	}
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	if name == "" {
		return "Working"
	}
	return "Using " + name
}
