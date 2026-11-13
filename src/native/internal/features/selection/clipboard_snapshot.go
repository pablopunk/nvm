package selection

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// clipboardSnapshot owns the displaced payload and the temporary write that may restore it.
type clipboardSnapshot struct {
	text    string
	full    platform.ClipboardSnapshot
	owner   platform.ClipboardSnapshotter
	lease   platform.ClipboardOwnership
	written *string
}

func (r *Reader) snapshot() (clipboardSnapshot, error) {
	if owner, ok := r.clip.(platform.ClipboardSnapshotter); ok {
		if pending := r.pending; pending != nil && pending.owner != nil {
			owned, err := owner.StillOwns(pending.lease)
			if err != nil {
				return clipboardSnapshot{}, err
			}
			if owned {
				return clipboardSnapshot{full: pending.full, owner: owner}, nil
			}
		}
		full, err := owner.Snapshot()
		return clipboardSnapshot{full: full, owner: owner}, err
	}
	formats, err := r.clip.Formats()
	if err != nil && !errors.Is(err, platform.ErrUnsupported) {
		return clipboardSnapshot{}, err
	}
	for _, format := range formats {
		if isFormatted(format) {
			return clipboardSnapshot{}, ErrFormattedClipboard
		}
	}
	text, err := r.clip.ReadText()
	return clipboardSnapshot{text: text}, err
}

func (r *Reader) writeTemporary(snapshot *clipboardSnapshot, text string) error {
	if snapshot.owner != nil {
		lease, err := snapshot.owner.WriteTemporaryText(text)
		if err == nil {
			snapshot.lease = lease
		}
		return err
	}
	if err := writeConcealed(r.clip, text); err != nil {
		return err
	}
	snapshot.written = &text
	return nil
}

func (r *Reader) restore(snapshot clipboardSnapshot) error {
	if snapshot.owner != nil {
		if snapshot.lease == nil {
			return nil
		}
		return snapshot.owner.RestoreIfOwned(snapshot.full, snapshot.lease)
	}
	if snapshot.written == nil {
		return nil
	}
	current, err := r.clip.ReadText()
	if err != nil || current != *snapshot.written {
		return err
	}
	if snapshot.text == "" {
		return r.clip.Clear()
	}
	return writeConcealed(r.clip, snapshot.text)
}
