package pi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// prepareTimeout bounds how long Prepare waits for the host to have its session ready.
const prepareTimeout = 30 * time.Second

// Prepare starts the host and has it create its session and resolve its route, so the first reply does not pay for
// them. It sends no prompt. It does nothing while a reply runs, since that reply already owns a ready session.
func (e *Engine) Prepare(ctx context.Context) error {
	if !e.mu.TryLock() {
		return nil
	}
	defer e.mu.Unlock()
	access, err := e.opts.Access()
	if err != nil {
		return err
	}
	if err := e.ensureLocked(ctx); err != nil {
		return err
	}
	message := e.message("prepare", fmt.Sprintf("prepare-%d", e.nextIDLocked()), access)
	message.Chars = e.requestChars(message)
	if err := e.send(message); err != nil {
		e.stopLocked()
		return fmt.Errorf("the assistant could not start: %w", err)
	}
	return e.awaitPrepared(ctx)
}

// awaitPrepared reads the host's events until it says its session is ready or it fails. A host that does not answer is
// stopped, since a later reply starts a new one.
func (e *Engine) awaitPrepared(ctx context.Context) error {
	timeout := time.After(prepareTimeout)
	for {
		select {
		case <-ctx.Done():
			e.stopLocked()
			return ctx.Err()
		case <-timeout:
			e.stopLocked()
			return errors.New("the assistant took too long to get ready")
		case line, ok := <-e.lines:
			if !ok {
				e.stopLocked()
				return errors.New("the assistant stopped unexpectedly")
			}
			var event hostEvent
			if json.Unmarshal(line, &event) != nil {
				continue
			}
			switch event.Type {
			case "prepared":
				return nil
			case "error":
				return hostError(event)
			}
		}
	}
}
