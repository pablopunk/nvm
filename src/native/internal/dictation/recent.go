package dictation

import "github.com/pablopunk/nvm/native/internal/dictation/recordings"

// recentKept is how many transcribed dictations keep their audio, so a transcript that came out wrong can be replayed
// through the pipeline to find out why. Older ones are deleted as new ones arrive.
const recentKept = 5

// recentKeeper is the recorder the engine gives the service when it keeps recent audio: a dictation whose transcript
// is in moves to the recent folder instead of being deleted. The history never lists that folder.
type recentKeeper struct {
	*recordings.Store
	recent *recordings.Store
}

// Remove moves the dictation's audio to the recent folder, then drops it from the unfinished ones. Keeping a copy is
// best effort: a copy that fails never keeps the audio among the unfinished dictations.
func (k recentKeeper) Remove(id string) error {
	if segments, err := k.Load(id); err == nil && len(segments) > 0 {
		if k.recent.Save(id, segments) == nil {
			_ = k.recent.KeepNewest(recentKept)
		}
	}
	return k.Store.Remove(id)
}
