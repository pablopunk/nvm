package extjobs

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// fileWatch is a files.changed trigger the host watches: the event it emits, the roots it watches and the filter an
// event must pass. trigger is the declared trigger, handed to the run as its launch context.
type fileWatch struct {
	event   string
	roots   []string
	filter  fileFilter
	trigger api.Trigger
}

// mapTriggers turns a command's declared triggers into scheduler triggers and the folder watches they need. A trigger
// the host cannot run yet is left out; the rest of the command still runs.
func mapTriggers(jobID string, declared []api.Trigger, home string) ([]jobs.Trigger, []fileWatch) {
	var triggers []jobs.Trigger
	var watches []fileWatch
	for index, trigger := range declared {
		mapped, watch, ok := mapTrigger(jobID, index, trigger, home)
		if !ok {
			continue
		}
		triggers = append(triggers, mapped)
		if watch != nil {
			watches = append(watches, *watch)
		}
	}
	return triggers, watches
}

// mapTrigger maps one declared trigger. It reports false for a trigger type the host does not run.
func mapTrigger(jobID string, index int, trigger api.Trigger, home string) (jobs.Trigger, *fileWatch, bool) {
	debounce := millis(trigger.DebounceMs)
	switch trigger.Type {
	case api.TriggerStartup:
		return jobs.Trigger{Kind: jobs.TriggerStartup, Delay: millis(trigger.DelayMs)}, nil, true
	case api.TriggerInterval:
		every, err := time.ParseDuration(trigger.Every)
		if err != nil {
			return jobs.Trigger{}, nil, false
		}
		return jobs.Trigger{Kind: jobs.TriggerInterval, Every: every, Delay: millis(trigger.DelayMs)}, nil, true
	case api.TriggerWake:
		return eventTrigger(jobs.EventWake, debounce), nil, true
	case api.TriggerLogin:
		return eventTrigger(jobs.EventLogin, debounce), nil, true
	case api.TriggerClipboardChanged:
		return eventTrigger(ClipboardChangedEvent, debounce), nil, true
	case api.TriggerAppFrontmostChange:
		return eventTrigger(FrontmostChangedEvent, debounce), nil, true
	case api.TriggerFilesChanged:
		watch := fileWatch{
			event:   filesEventName(jobID, index),
			roots:   expandRoots(trigger.Roots, home),
			filter:  newFileFilter(trigger),
			trigger: trigger,
		}
		return eventTrigger(watch.event, debounce), &watch, true
	}
	return jobs.Trigger{}, nil, false
}

// filesEventName is the event one files.changed trigger emits. It names the job and the trigger's position, so an
// event reaches only the job that declared it.
func filesEventName(jobID string, index int) string {
	return "files.changed:" + jobID + ":" + strconv.Itoa(index)
}

func eventTrigger(name string, debounce time.Duration) jobs.Trigger {
	return jobs.Trigger{Kind: jobs.TriggerEvent, Event: name, Debounce: debounce}
}

func millis(value int64) time.Duration {
	return time.Duration(value) * time.Millisecond
}

// expandRoots expands a leading "~" and keeps the absolute roots, cleaned.
func expandRoots(roots []string, home string) []string {
	var expanded []string
	for _, root := range roots {
		if root == "~" || strings.HasPrefix(root, "~/") {
			root = filepath.Join(home, root[1:])
		}
		if filepath.IsAbs(root) {
			expanded = append(expanded, filepath.Clean(root))
		}
	}
	return expanded
}
