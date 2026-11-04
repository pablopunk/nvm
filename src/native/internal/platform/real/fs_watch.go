package real

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// watchBuffer is how many events may wait for a consumer before the relay blocks on it.
const watchBuffer = 64

// startWatch watches the tree under name with MyGo, without the entries skip returns true for, and relays its batches
// as single events. The entry limit is MyGo's default for the platform: a macOS watch keeps a larger snapshot because
// one FSEvents stream covers the whole tree. The channel closes when ctx is done or MyGo stops the watch (for example
// when the watched folder disappears).
func startWatch(ctx context.Context, name string, skip func(rel string, isDir bool) bool) (<-chan platform.Event, error) {
	watcher, err := mygo.WatchFiles(ctx, name, mygo.FileWatchOptions{Recursive: true, Ignore: skip})
	if err != nil {
		if errors.Is(err, mygo.ErrWatchLimit) {
			return nil, fmt.Errorf("%w: %w", platform.ErrWatchLimit, err)
		}
		return nil, err
	}
	events := make(chan platform.Event, watchBuffer)
	go relayWatchEvents(ctx, watcher, name, events)
	return events, nil
}

// relayWatchEvents forwards each change until the watch ends, then closes out and the watcher. A watch that stopped
// at its entry limit first reports an overflow for its folder, so the consumer knows changes were lost.
func relayWatchEvents(ctx context.Context, watcher *mygo.FileWatcher, root string, out chan<- platform.Event) {
	defer close(out)
	defer func() { _ = watcher.Close() }()
	for {
		batch, err := watcher.Next(ctx)
		if err != nil {
			if errors.Is(err, mygo.ErrWatchLimit) {
				relayOverflow(ctx, root, out)
			}
			return
		}
		for _, change := range batch {
			event, ok := convertWatchChange(root, change)
			if !ok {
				continue
			}
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}
}

// convertWatchChange maps a MyGo change to the port's event. Hints, which say only that something under
// the folder changed, become writes so the consumer reads the folder again.
func convertWatchChange(root string, change mygo.FileEvent) (platform.Event, bool) {
	event := platform.Event{Path: filepath.Join(root, filepath.FromSlash(change.Path)), IsDir: change.IsDir}
	switch change.Op {
	case mygo.FileCreate:
		event.Op = platform.EventCreate
	case mygo.FileWrite, mygo.FileHint:
		event.Op = platform.EventWrite
	case mygo.FileRemove:
		event.Op = platform.EventRemove
	case mygo.FileRename:
		event.Op = platform.EventRename
		event.OldPath = filepath.Join(root, filepath.FromSlash(change.OldPath))
	default:
		return platform.Event{}, false
	}
	return event, true
}

// relayOverflow tells the consumer that a watch lost changes for root, unless the watch was cancelled first.
func relayOverflow(ctx context.Context, root string, out chan<- platform.Event) {
	select {
	case out <- platform.Event{Op: platform.EventOverflow, Path: root, IsDir: true}:
	case <-ctx.Done():
	}
}
