package api

import (
	"errors"
	"fmt"
	"time"
)

// TriggerType names a host event that can run a command or refresh a menu bar item. The host
// owns scheduling, enablement, no-overlap, backoff and diagnostics; the extension only declares.
type TriggerType string

const (
	TriggerStartup            TriggerType = "startup"
	TriggerInterval           TriggerType = "interval"
	TriggerFilesChanged       TriggerType = "files.changed"
	TriggerClipboardChanged   TriggerType = "clipboard.changed"
	TriggerAppFrontmostChange TriggerType = "app.frontmost.changed"
	TriggerWake               TriggerType = "wake"
	TriggerLogin              TriggerType = "login"
)

// FileKindFilter narrows files.changed events.
type FileKindFilter string

const (
	FileKindFile  FileKindFilter = "file"
	FileKindImage FileKindFilter = "image"
	FileKindVideo FileKindFilter = "video"
	FileKindMedia FileKindFilter = "media"
)

// minimumInterval floors interval triggers so that a typo cannot schedule a busy loop.
const minimumInterval = time.Second

// Trigger is one declarative trigger intent. Fields are read according to Type; unused fields
// must stay zero so that Validate can reject mistakes such as a debounce on a startup trigger.
type Trigger struct {
	Type    TriggerType `json:"type"`
	DelayMs int64       `json:"delayMs,omitempty"`
	// Every is a Go duration string such as "30s" or "5m". Interval only; at least one second.
	Every string `json:"every,omitempty"`
	// Roots, DebounceMs, IncludeHidden, Extensions, FileKind and Ignore configure files.changed.
	// The host expands "~" in roots and watches with a bounded launch context.
	Roots         []string       `json:"roots,omitempty"`
	DebounceMs    int64          `json:"debounceMs,omitempty"`
	IncludeHidden bool           `json:"includeHidden,omitempty"`
	Extensions    []string       `json:"extensions,omitempty"`
	FileKind      FileKindFilter `json:"fileKind,omitempty"`
	Ignore        []string       `json:"ignore,omitempty"`
}

// Validate checks the trigger type and the fields that type requires.
func (t Trigger) Validate() error {
	switch t.Type {
	case TriggerStartup, TriggerWake, TriggerLogin:
		return nil
	case TriggerInterval:
		return validateInterval(t.Every)
	case TriggerFilesChanged:
		return validateFilesChanged(t)
	case TriggerClipboardChanged, TriggerAppFrontmostChange:
		return nil
	}
	return fmt.Errorf("unknown trigger type %q", t.Type)
}

func validateInterval(every string) error {
	if every == "" {
		return errors.New("interval trigger needs every, e.g. \"5m\"")
	}
	duration, err := time.ParseDuration(every)
	if err != nil {
		return fmt.Errorf("interval %q is not a duration: %v", every, err)
	}
	if duration < minimumInterval {
		return fmt.Errorf("interval %q is below the one second minimum", every)
	}
	return nil
}

func validateFilesChanged(t Trigger) error {
	if len(t.Roots) == 0 {
		return errors.New("files.changed needs at least one root")
	}
	if t.FileKind != "" && !oneOf(t.FileKind, FileKindFile, FileKindImage, FileKindVideo, FileKindMedia) {
		return fmt.Errorf("unknown file kind %q", t.FileKind)
	}
	return nil
}
