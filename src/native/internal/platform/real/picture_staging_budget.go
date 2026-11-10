package real

import (
	"context"
	"errors"
	"sync"
)

const (
	pictureSourceLimit    int64 = 256 << 20
	pictureStagingLimit   int64 = 512 << 20
	pictureStagingReserve int64 = 256 << 20
)

var errPictureStagingSpace = errors.New("file picture staging exceeds available space")
var pictureStaging = newPictureStagingBudget(pictureStagingLimit)

type pictureStagingBudget struct {
	mu          sync.Mutex
	used, limit int64
	changed     chan struct{}
}

func newPictureStagingBudget(limit int64) *pictureStagingBudget {
	return &pictureStagingBudget{limit: limit, changed: make(chan struct{})}
}

// reserve admits a whole copy or none; callers retain its reservation until the staged file is removed.
func (b *pictureStagingBudget) reserve(ctx context.Context, bytes int64, available func() (int64, error)) (func(), error) {
	if bytes < 0 || bytes > b.limit {
		return nil, errPictureStagingSpace
	}
	for {
		b.mu.Lock()
		if err := ctx.Err(); err != nil {
			b.mu.Unlock()
			return nil, err
		}
		if bytes <= b.limit-b.used {
			free, err := available()
			if err != nil {
				b.mu.Unlock()
				return nil, err
			}
			// Include not-yet-written reservations, even if some copies already consume disk space.
			if free < pictureStagingReserve || bytes > free-pictureStagingReserve-b.used {
				b.mu.Unlock()
				return nil, errPictureStagingSpace
			}
			b.used += bytes
			b.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					b.mu.Lock()
					b.used -= bytes
					close(b.changed)
					b.changed = make(chan struct{})
					b.mu.Unlock()
				})
			}, nil
		}
		changed := b.changed
		b.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}
