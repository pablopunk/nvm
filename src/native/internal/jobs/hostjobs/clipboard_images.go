package hostjobs

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// Clipboard image cleanup keeps the stored pictures that no history entry references. It runs an hour apart, first
// thirty seconds after launch, as the 1.x host job did.
const (
	ClipboardImageCleanupID      = "clipboard.images.gc"
	ClipboardImageCleanupEvery   = time.Hour
	ClipboardImageCleanupDelay   = 30 * time.Second
	ClipboardImageCleanupTimeout = 20 * time.Second
)

// ImageSweeper is what the cleanup needs from the picture store. clipboard.ImageStore satisfies it.
type ImageSweeper interface {
	GC(refs clipboard.GCRefs) clipboard.GCResult
}

// ClipboardImageCleanup returns the job definition. A sweep that the feature refuses (an untrusted history, a failed
// listing) deletes nothing and is not a failure of the job.
func ClipboardImageCleanup(images ImageSweeper, refs clipboard.GCRefs) jobs.Definition {
	return jobs.Definition{
		ID:      ClipboardImageCleanupID,
		Title:   "Clipboard Image Cleanup",
		Owner:   jobs.OwnerHost,
		Scope:   "cache",
		Triggers: []jobs.Trigger{{
			Kind:  jobs.TriggerInterval,
			Every: ClipboardImageCleanupEvery,
			Delay: ClipboardImageCleanupDelay,
		}},
		Budget: jobs.Budget{Timeout: ClipboardImageCleanupTimeout},
		Run: func(_ context.Context, _ jobs.Run) error {
			images.GC(refs)
			return nil
		},
	}
}
