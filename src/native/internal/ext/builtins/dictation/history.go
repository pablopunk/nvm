package dictation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
)

// Copy of the history view: its title, search placeholder and empty state, as 1.x showed them.
const (
	historyPlaceholder = "Search dictation history"
	historyEmptyTitle  = "No dictation history"
	historyEmptyHint   = "Completed dictations will appear here."
	recordingTitle     = "Unfinished dictation"
	clearTitle         = "Clear History"
	clearMessage       = "Delete all transcripts and saved recordings? This cannot be undone."
	clearLabel         = "Clear history"
	deleteRecordHint   = "Delete this saved audio? It cannot be recovered."
	deleteRecordLabel  = "Delete recording"
	deleteTextHint     = "Delete this transcript? This cannot be undone."
	deleteTextLabel    = "Delete transcript"
	timeLayout         = "2006-01-02 15:04"
	previewTitle       = "Dictation"
)

// readHistory returns the saved transcripts, newest first. Until the built-in saves its own, the app's legacy list answers.
func (e Extension) readHistory(ctx api.Context) []history.Entry {
	var saved []history.Entry
	if found, err := ctx.Storage().Get(storageHistoryKey, &saved); err == nil && found {
		return history.Clean(saved)
	}
	return history.Clean(e.legacy.History)
}

// saveTranscript adds a finished dictation to the history. A save that fails is logged, and the dictation still pastes.
func (e Extension) saveTranscript(ctx api.Context, text string) {
	if err := e.addTranscript(ctx, text); err != nil {
		ctx.Logs().Log(api.LogWarn, "Dictation history save failed", map[string]string{"message": err.Error()})
	}
}

func (e Extension) addTranscript(ctx api.Context, text string) error {
	now := ctx.Clock().Now()
	entry := history.Entry{ID: "dictation-" + strconv.FormatInt(now.UnixNano(), 36), Text: text, CreatedAt: now.UnixMilli()}
	return writeHistory(ctx, history.Add(e.readHistory(ctx), entry))
}

func writeHistory(ctx api.Context, entries []history.Entry) error {
	return ctx.Storage().Set(storageHistoryKey, entries)
}

// historyView is the Dictation History list: the unfinished dictations with Retry, then the transcripts with Copy and
// Delete. A list over the cap drops the oldest, as the history keeps MaxEntries of each kind.
func (e Extension) historyView(ctx api.Context) api.View {
	entries, saved := e.retainedHistory(ctx)
	items := make([]api.Item, 0, len(saved)+len(entries))
	for _, recording := range saved {
		items = append(items, recordingItem(recording))
	}
	for _, entry := range entries {
		items = append(items, transcriptItem(entry))
	}
	view := api.ListView(historyTitle, items...)
	view.Subtitle = fmt.Sprintf("%d transcripts · %d saved recordings", len(entries), len(saved))
	view.Placeholder = historyPlaceholder
	view.EmptyView = &api.EmptyState{Title: historyEmptyTitle, Subtitle: historyEmptyHint}
	if len(items) > 0 {
		clearAll := clearAction()
		view.Actions = []api.Action{clearAll}
		view.ActionPanel = api.Panel(api.Section("", clearAll))
	}
	return view
}

// retainedHistory returns the transcripts and saved dictations to show. Together they stay within the cap; the dictations
// past it are deleted, and so are the transcripts.
func (e Extension) retainedHistory(ctx api.Context) ([]history.Entry, []api.SavedDictation) {
	entries := e.readHistory(ctx)
	saved, err := ctx.Dictation().Recordings()
	if err != nil {
		saved = nil
	}
	kept := keptIDs(entries, saved)
	var dropped []api.SavedDictation
	var shownRecordings []api.SavedDictation
	for _, recording := range saved {
		if kept[recording.ID] {
			shownRecordings = append(shownRecordings, recording)
		} else {
			dropped = append(dropped, recording)
		}
	}
	for _, recording := range dropped {
		_ = ctx.Dictation().DeleteRecording(recording.ID)
	}
	var shownEntries []history.Entry
	for _, entry := range entries {
		if kept[entry.ID] {
			shownEntries = append(shownEntries, entry)
		}
	}
	if len(shownEntries) != len(entries) {
		_ = writeHistory(ctx, shownEntries)
	}
	return shownEntries, shownRecordings
}

// keptIDs returns the ids within the cap, newest first across transcripts and saved dictations.
func keptIDs(entries []history.Entry, saved []api.SavedDictation) map[string]bool {
	type stamped struct {
		id        string
		createdAt int64
	}
	all := make([]stamped, 0, len(entries)+len(saved))
	for _, entry := range entries {
		all = append(all, stamped{entry.ID, entry.CreatedAt})
	}
	for _, recording := range saved {
		all = append(all, stamped{recording.ID, recording.CreatedAtMs})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].createdAt > all[j].createdAt })
	kept := map[string]bool{}
	for i, item := range all {
		if i < history.MaxEntries {
			kept[item.id] = true
		}
	}
	return kept
}

// recordingItem is the row of an unfinished dictation: Enter retries it, and its menu deletes the saved audio.
func recordingItem(recording api.SavedDictation) api.Item {
	retry := api.Run(refRetry, "Retry Transcription", map[string]string{argID: recording.ID})
	remove := api.Destructive(api.WithConfirmation(
		api.Run(refDeleteRecording, "Delete Recording", map[string]string{argID: recording.ID}),
		api.Confirmation{Message: deleteRecordHint, ConfirmLabel: deleteRecordLabel, Destructive: true},
	))
	return api.Item{
		ID: recording.ID, Title: recordingTitle, Subtitle: fmt.Sprintf("%s · %d audio segments", formatTime(recording.CreatedAtMs), recording.SegmentCount),
		Icon: "mic", Primary: &retry, Actions: []api.Action{remove},
	}
}

// transcriptItem is the row of a transcript: Enter opens it, and its menu copies or deletes it.
func transcriptItem(entry history.Entry) api.Item {
	copyAction := api.CopyText(copyTranscriptTitle, entry.Text, false)
	remove := api.Destructive(api.WithConfirmation(
		api.Run(refDeleteTranscript, "Delete Transcript", map[string]string{argID: entry.ID}),
		api.Confirmation{Message: deleteTextHint, ConfirmLabel: deleteTextLabel, Destructive: true},
	))
	open := api.PushView("View Transcript", transcriptPreview(entry))
	return api.Item{
		ID: entry.ID, Title: history.Title(entry.Text), Subtitle: formatTime(entry.CreatedAt), Icon: "file-text",
		Primary: &open, Actions: []api.Action{copyAction, remove},
	}
}

// transcriptPreview shows one transcript, with a copy action of its own.
func transcriptPreview(entry history.Entry) api.View {
	view := api.PreviewView(previewTitle, entry.Text)
	view.Subtitle = formatTime(entry.CreatedAt)
	view.Actions = []api.Action{api.CopyText(copyTranscriptTitle, entry.Text, false)}
	return view
}

func clearAction() api.Action {
	return api.Destructive(api.WithConfirmation(
		api.Run(refClearHistory, clearTitle, nil),
		api.Confirmation{Message: clearMessage, ConfirmLabel: clearLabel, Destructive: true},
	))
}

// formatTime is a saved time in the local zone, for the history's subtitles.
func formatTime(ms int64) string {
	return time.UnixMilli(ms).Format(timeLayout)
}

// openHistory shows the Dictation History list above the current page.
func (e Extension) openHistory(ctx api.Context) (api.Result, error) {
	return pushView(ctx, e.historyView(ctx)), nil
}

// retry transcribes a saved dictation again. The text joins the history and the audio is dropped; a failed or empty
// retry keeps the audio and says why.
func (e Extension) retry(ctx api.Context, id string) (api.Result, error) {
	text, err := ctx.Dictation().Retry(id)
	if err != nil {
		return toast(err.Error(), "error"), nil
	}
	if strings.TrimSpace(text) == "" {
		return toast("No speech detected", "error"), nil
	}
	if err := e.addTranscript(ctx, text); err != nil {
		return api.Result{}, err
	}
	if err := ctx.Dictation().DeleteRecording(id); err != nil {
		return api.Result{}, err
	}
	result := replaceView(ctx, e.historyView(ctx))
	result.Toast = &api.Toast{Message: "Transcription recovered", Tone: "success"}
	return result, nil
}

// deleteRecording removes a saved dictation's audio, and shows the list without it.
func (e Extension) deleteRecording(ctx api.Context, id string) (api.Result, error) {
	if err := ctx.Dictation().DeleteRecording(id); err != nil {
		return api.Result{}, err
	}
	return replaceView(ctx, e.historyView(ctx)), nil
}

// deleteTranscript removes one transcript the user confirmed, and shows the list without it.
func (e Extension) deleteTranscript(ctx api.Context, id string) (api.Result, error) {
	var kept []history.Entry
	for _, entry := range e.readHistory(ctx) {
		if entry.ID != id {
			kept = append(kept, entry)
		}
	}
	if err := writeHistory(ctx, kept); err != nil {
		return api.Result{}, err
	}
	result := replaceView(ctx, e.historyView(ctx))
	result.Toast = &api.Toast{Message: "Transcript deleted", Tone: "success"}
	return result, nil
}

// clearHistory removes every transcript and every saved dictation.
func (e Extension) clearHistory(ctx api.Context) (api.Result, error) {
	if err := writeHistory(ctx, nil); err != nil {
		return api.Result{}, err
	}
	saved, err := ctx.Dictation().Recordings()
	if err != nil {
		return api.Result{}, err
	}
	for _, recording := range saved {
		if err := ctx.Dictation().DeleteRecording(recording.ID); err != nil {
			return api.Result{}, err
		}
	}
	result := replaceView(ctx, e.historyView(ctx))
	result.Toast = &api.Toast{Message: "Dictation history cleared", Tone: "success"}
	return result, nil
}

func toast(message, tone string) api.Result {
	return api.Result{Toast: &api.Toast{Message: message, Tone: tone}}
}
