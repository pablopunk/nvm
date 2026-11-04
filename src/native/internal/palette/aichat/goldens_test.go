package aichat

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// pageGoldens renders a chat page at the palette's size. Each golden is compared in light and dark, at 1x and 2x.
var pageGoldens = golden.Options{Group: "aichat", Width: goldenWidth, Height: goldenHeight}

// streamingGoldens lets a reply that is still arriving finish revealing before its frame is compared.
var streamingGoldens = golden.Options{Group: "aichat", Width: goldenWidth, Height: goldenHeight, Advance: 10 * time.Second}

// longMarkdown is a reply with every block the transcript must render: headings, emphasis, a list, a code block and a
// table.
const longMarkdown = "## Release notes\n\nThe palette got three changes this week, all behind the **same** shortcut.\n\n" +
	"- Clipboard history shows *previews* for images\n- Search ranks `apps` above files\n- Escape stops a reply in flight\n\n" +
	"```go\nfunc Summon() error {\n\treturn palette.Show()\n}\n```\n\n" +
	"| Area | Status | Owner |\n| --- | --- | --- |\n| Clipboard | shipped | Pat |\n| Files | in review | Sam |\n"

// noticeFor returns the notice the service would write for err, so the goldens show the shipped copy.
func noticeFor(err error, partial string) chat.Notice {
	return chat.NoticeFor(err, partial)
}

// TestTranscriptGoldens renders a short exchange: a question and a finished answer.
func TestTranscriptGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan a weekend", chat.TierFast,
		user("Summarize what changed in the palette this week."),
		assistant("The palette got faster and clearer. Search ranks apps first."))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "transcript", r.palette.View, pageGoldens)
}

// TestMarkdownGoldens renders a long reply with a heading, a list, a code block and a table.
func TestMarkdownGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Release notes", chat.TierFast, user("Write the release notes."), assistant(longMarkdown))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "markdown", r.palette.View, pageGoldens)
}

// TestStreamingGoldens renders a reply that is still arriving, with the spinner-free text in the last line.
func TestStreamingGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Next steps", chat.TierFast,
		user("And what is next?"),
		streaming("Next up: **snippets** with keywords and a faster emoji grid"))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "streaming", r.palette.View, streamingGoldens)
}

// TestStoppedGoldens renders a reply the user stopped: the text it had stays, marked as stopped.
func TestStoppedGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Stopped reply", chat.TierFast,
		user("Explain the budgets."),
		stopped("The budgets are measured on the reference Mac and on the Linux box, and each one has a p95 target"))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "stopped", r.palette.View, pageGoldens)
}

// TestLimitNoticeGoldens renders the notices a refused turn leaves behind, from the account side: credits, the
// free spend cap, a rate limit and a request too large.
func TestLimitNoticeGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Limits", chat.TierFast,
		user("Draft the summary."),
		notice(noticeFor(&backend.Error{Kind: backend.KindInsufficientCredits, Status: 402,
			CreditsInfo: &backend.CreditsInfo{EstimatedCredits: 95, Balance: 81}}, "")),
		notice(noticeFor(&backend.Error{Kind: backend.KindRejected, Status: 402, Code: "free_spend_cap_reached"}, "")),
		notice(noticeFor(&backend.Error{Kind: backend.KindRateLimited, Status: 429, RetryAfter: 30 * time.Second}, "")),
		notice(noticeFor(&backend.Error{Kind: backend.KindRejected, Status: 413, Code: "prompt_too_large"}, "")))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "notices-limits", r.palette.View, pageGoldens)
}

// TestFailureNoticeGoldens renders the notices for connection, route and reply failures.
func TestFailureNoticeGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Failures", chat.TierFast,
		user("Summarize the inbox."),
		stopped("Here is the start of a summary"),
		notice(noticeFor(&backend.Error{Kind: backend.KindBadResponse}, "Here is the start")),
		notice(noticeFor(&backend.Error{Kind: backend.KindOffline}, "")),
		notice(noticeFor(&backend.Error{Kind: backend.KindTimeout}, "")),
		notice(noticeFor(chat.ErrRouteUnsupported, "")))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "notices-failures", r.palette.View, pageGoldens)
}

// TestEmptyChatGoldens renders a new chat for a signed-in account, and for an account that is not signed in.
func TestEmptyChatGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "empty", r.palette.View, pageGoldens)

	r.signed = false
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "empty-signed-out", r.palette.View, pageGoldens)

	r.sign = auth.SignInState{Phase: auth.PhasePending, Code: "CODE-123", VerificationURL: "https://www.nvm.fyi/device"}
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "empty-signed-out-pending", r.palette.View, pageGoldens)
}

// TestHistoryGoldens renders the list of saved conversations, most recent first, under the new-chat row.
func TestHistoryGoldens(t *testing.T) {
	r := newRig(t, nil)
	r.chatWith("Plan a weekend in Lisbon", chat.TierFast, user("Plan it"), assistant("Day one: the old town."))
	r.clock.Advance(3 * time.Hour)
	r.chatWith("Explain goroutine leaks", chat.TierSmart, user("Explain it"), assistant("A leak is a goroutine that never returns."))
	r.open(HistoryView(r.deps()))
	golden.AssertMatrix(t, "history", r.palette.View, pageGoldens)
}

// TestThinkingGoldens renders a reply that has started but has no visible text yet: the thinking state.
func TestThinkingGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan a trip", chat.TierSmart,
		user("Plan a three day trip to Lisbon."),
		streaming(""))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "thinking", r.palette.View, pageGoldens)
}

// TestReasoningGoldens renders a finished reply whose Thinking block is closed, then the same block opened.
func TestReasoningGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan a trip", chat.TierSmart,
		user("Plan a three day trip to Lisbon."),
		reasoned("Day one: the old town. Day two: Sintra. Day three: the coast.", "Weighing a pace of two sights a day against the hills, then the coast."))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "reasoning-closed", r.palette.View, pageGoldens)

	opened := newPage(r.deps(), record.ID)
	opened.thinkingOpen[1] = true
	r.open(opened.view())
	golden.AssertMatrix(t, "reasoning-open", r.palette.View, pageGoldens)
}

// TestBuilderWritingGoldens renders a builder turn that wrote its extension: the write's tool row is done, and the
// check of the extension is the step under the transcript.
func TestBuilderWritingGoldens(t *testing.T) {
	r := newRig(t, nil)
	written := chat.Message{Author: chat.AuthorTool, Status: chat.StatusComplete, At: goldenNow, Text: "Installed and enabled \"Clock\".",
		Tool: &chat.ToolRun{Name: "write_extension", Label: "Writing extension", Input: "user.clock", Outcome: "Installed and enabled"}}
	record := r.builderWith("", user("A clock in the palette."), written)
	r.service.SetStep(record.ID, "Checking…")
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "builder-writing", r.palette.View, streamingGoldens)
}

// splitGoldens renders a page split beside its preview at the size 1.x gives that window.
var splitGoldens = golden.Options{Group: "aichat", Width: palette.SplitWidth, Height: palette.SplitHeight}

// TestBuilderPreviewGoldens renders a builder chat with the extension's preview open beside it: the extension's rows are
// on the right of the transcript, at the split size, and the focused preview shows its edge.
func TestBuilderPreviewGoldens(t *testing.T) {
	titles := []string{"Local time", "Today's date"}
	r, side := previewRigWith(t, &titles)
	record := r.builderWith("user.clock", user("A clock in the palette."), assistant("Installed the clock. It is not run yet."))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "builder-preview", r.palette.View, splitGoldens)

	side.Focus()
	golden.AssertMatrix(t, "builder-preview-focused", r.palette.View, splitGoldens)
}
