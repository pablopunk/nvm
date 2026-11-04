package app

import (
	"fmt"
	"strings"
	"time"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/history"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// dictationHistoryView lists the unfinished dictations, then the transcripts, newest first. Unfinished dictations can
// be transcribed again; each row can be copied, or deleted after a confirmation.
func (p *App) dictationHistoryView() *palette.View {
	return &palette.View{
		Title:       "Dictation History",
		Placeholder: "Search dictation history",
		EmptyTitle:  "No dictation history",
		EmptySub:    "Completed dictations will appear here.",
		Search: func(q string) []search.Item {
			query := strings.ToLower(strings.TrimSpace(q))
			var items []search.Item
			unfinished := p.unfinishedDictations()
			for _, recording := range unfinished {
				if matchesQuery(query, "Unfinished dictation") {
					items = append(items, p.unfinishedRow(recording))
				}
			}
			entries := p.dictationTranscripts().Entries()
			for _, entry := range entries {
				if matchesQuery(query, entry.Text) {
					items = append(items, p.transcriptRow(entry))
				}
			}
			if query == "" && (len(unfinished) > 0 || len(entries) > 0) {
				items = append(items, p.clearHistoryRow())
			}
			return items
		},
	}
}

func matchesQuery(query, text string) bool {
	return query == "" || strings.Contains(strings.ToLower(text), query)
}

// unfinishedRow is a dictation whose text is not in: its audio is kept, so Retry transcribes it again.
func (p *App) unfinishedRow(recording recordings.Recording) search.Item {
	return search.Item{
		Title:    "Unfinished dictation",
		Subtitle: fmt.Sprintf("%s · %d audio segments", historyTime(recording.CreatedAt), recording.SegmentCount),
		Icon:     "mic",
		Actions: []search.Action{
			{Title: "Retry Transcription", Dismiss: search.DismissNever, Run: func(h search.Host) {
				p.recoverDictation(recording.ID)
			}},
			{Title: "Delete Recording", Dismiss: search.DismissNever, Run: func(h search.Host) {
				h.Push(palette.Confirm("Delete recording", "Delete this saved audio? It cannot be recovered.", func() {
					p.deleteRecording(recording.ID)
					p.Refresh()
				}))
			}},
		},
	}
}

// transcriptRow is one finished dictation: its title is the start of the text, and its first action copies it.
func (p *App) transcriptRow(entry history.Entry) search.Item {
	return search.Item{
		Title:    history.Title(entry.Text),
		Subtitle: historyTime(entry.CreatedAt),
		Icon:     "file-text",
		Actions: []search.Action{
			{Title: "Copy Transcript", Dismiss: search.DismissNever, Run: func(h search.Host) {
				p.copyText(h, entry.Text)
			}},
			{Title: "Delete Transcript", Dismiss: search.DismissNever, Run: func(h search.Host) {
				h.Push(palette.Confirm("Delete transcript", "Delete this transcript? This cannot be undone.", func() {
					p.forgetTranscript(entry.ID)
					p.Refresh()
				}))
			}},
		},
	}
}

func (p *App) clearHistoryRow() search.Item {
	return search.Item{
		Title:    "Clear History",
		Subtitle: "Delete all transcripts and saved recordings",
		Icon:     "trash-2",
		Actions: []search.Action{{Title: "Clear History", Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(palette.Confirm("Clear history", "Delete all transcripts and saved recordings? This cannot be undone.", func() {
				p.clearTranscripts()
				for _, recording := range p.unfinishedDictations() {
					p.deleteRecording(recording.ID)
				}
				p.Toast("Dictation history cleared")
				p.Refresh()
			}))
		}}},
	}
}

// recoverDictation transcribes a saved dictation again. The text joins the history and the audio is dropped; a
// failure or silence keeps the audio for another try.
func (p *App) recoverDictation(id string) {
	p.dictationFlowFor().Recover(id, func(text string, err error) {
		switch {
		case err != nil:
			p.Toast(dict.FailureMessage(err))
		case strings.TrimSpace(text) == "":
			p.Toast("No speech detected")
		default:
			p.rememberDictation(text)
			p.deleteRecording(id)
			p.Toast("Transcription recovered")
		}
		p.Refresh()
	})
}

// unfinishedDictations lists the saved dictations, or none when the store cannot be read.
func (p *App) unfinishedDictations() []recordings.Recording {
	store := p.dictationRecordingStore()
	if store == nil {
		return nil
	}
	list, err := store.List()
	if err != nil {
		return nil
	}
	return list
}

func (p *App) deleteRecording(id string) {
	if store := p.dictationRecordingStore(); store != nil {
		_ = store.Remove(id)
	}
}

func historyTime(unixMillis int64) string {
	return time.UnixMilli(unixMillis).Format("Jan 2, 15:04")
}
