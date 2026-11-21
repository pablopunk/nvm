package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/dictation/audiomute"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// dictationMuteFile is the journal of the output mute in progress, in the data folder.
const dictationMuteFile = "dictation-audio-mute.json"

// fileMuteJournal keeps the mute journal in one file, so a crash leaves it for the next start.
type fileMuteJournal struct {
	files platform.FS
	path  string
}

type muteJournalEntry struct {
	WasMuted bool `json:"wasMuted"`
}

func (j fileMuteJournal) Save(wasMuted bool) error {
	data, err := json.Marshal(muteJournalEntry{WasMuted: wasMuted})
	if err != nil {
		return err
	}
	return j.files.WriteFile(j.path, data, 0o600)
}

func (j fileMuteJournal) Load() (bool, bool, error) {
	data, err := j.files.ReadFile(j.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	var entry muteJournalEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return false, false, err
	}
	return entry.WasMuted, true, nil
}

func (j fileMuteJournal) Clear() error {
	if err := j.files.Remove(j.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// dictationMuter returns the output mute of the dictations, or nil where the OS cannot mute the output.
func (p *App) dictationMuter() *audiomute.Muter {
	if p.ports.OutputMute == nil {
		return nil
	}
	journal := fileMuteJournal{files: p.ports.FS, path: filepath.Join(dataDir(), dictationMuteFile)}
	return audiomute.New(p.ports.OutputMute, journal)
}
