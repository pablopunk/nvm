package aichat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// toolEntry builds a tool row for the goldens: a call named name, shown with label and input, in status with outcome.
func toolEntry(status chat.Status, name, label, input, outcome string) chat.Message {
	return chat.Message{Author: chat.AuthorTool, Status: status, At: goldenNow, Tool: &chat.ToolRun{
		CallID: name, Name: name, Label: label, Input: input, Outcome: outcome,
	}}
}

// TestToolRowGoldens renders a reply that used tools: one row in each state a call can end in, in light and dark.
func TestToolRowGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Find my notes", chat.TierFast,
		user("Find my meeting notes from March and open the folder."),
		assistant("Let me look."),
		toolEntry(chat.StatusComplete, "search_files", "Searching files", "meeting notes", "Found 3 files"),
		toolEntry(chat.StatusComplete, "list_folder", "Listing a folder", "~/Documents", "12 items"),
		toolEntry(chat.StatusFailed, "read_file", "Reading a file", "~/Documents/big.bin", "Failed: that file is not text"),
		toolEntry(chat.StatusRefused, "read_file", "Reading a file", "~/.ssh/id_ed25519", "Refused: access to credential locations is blocked"),
		toolEntry(chat.StatusCancelled, "trash_file", "Moving to the Trash", "~/Documents/old.txt", ""),
		toolEntry(chat.StatusRunning, "open_app", "Opening an app", "Notes", ""),
	)
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "tool-rows", r.palette.View, pageGoldens)
}

// TestToolAwaitingGoldens renders a call that waits for the user, with the footer that names the keys that answer it.
func TestToolAwaitingGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Clean up", chat.TierFast,
		user("Trash the old export."),
		toolEntry(chat.StatusAwaiting, "trash_file", "Moving to the Trash", "~/Downloads/export-2025.zip", ""),
	)
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "tool-awaiting", r.palette.View, pageGoldens)
}
