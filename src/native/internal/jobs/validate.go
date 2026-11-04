package jobs

import (
	"fmt"
)

// validate reports the first mistake that would make the definition misbehave, wrapping ErrInvalidDefinition.
func (d Definition) validate() error {
	switch {
	case d.ID == "":
		return invalid("a job needs an id")
	case d.Title == "":
		return invalid("job %s needs a title", d.ID)
	case d.Run == nil:
		return invalid("job %s needs a run function", d.ID)
	case d.Owner != "" && d.Owner != OwnerHost && d.Owner != OwnerExtension:
		return invalid("job %s has unknown owner %q", d.ID, d.Owner)
	case d.Budget.Timeout < 0 || d.Budget.MaxRuns < 0 || d.Budget.Window < 0:
		return invalid("job %s has a negative budget", d.ID)
	case d.Budget.MaxRuns > 0 && d.Budget.Window <= 0:
		return invalid("job %s limits runs but gives no window", d.ID)
	}
	for _, trigger := range d.Triggers {
		if err := trigger.validate(); err != nil {
			return invalid("job %s: %v", d.ID, err)
		}
	}
	return nil
}

// validate checks a trigger's kind and that only the fields its kind uses are set.
func (t Trigger) validate() error {
	switch t.Kind {
	case TriggerStartup:
		if t.Every != 0 || t.Event != "" || t.Debounce != 0 || t.MaxWait != 0 || t.Missed != "" || t.Delay < 0 {
			return fmt.Errorf("startup takes only a delay")
		}
	case TriggerInterval:
		return t.validateInterval()
	case TriggerEvent:
		if t.Event == "" || t.Every != 0 || t.Delay != 0 || t.Debounce < 0 || t.MaxWait < 0 || t.Missed != "" {
			return fmt.Errorf("event %q needs a name and only debounce and maxWait", t.Event)
		}
	default:
		return fmt.Errorf("unknown trigger kind %q", t.Kind)
	}
	return nil
}

func (t Trigger) validateInterval() error {
	if t.Every < MinInterval {
		return fmt.Errorf("interval %s is below the %s minimum", t.Every, MinInterval)
	}
	if t.Delay < 0 || t.Event != "" || t.Debounce != 0 || t.MaxWait != 0 {
		return fmt.Errorf("interval takes only every, delay and missed")
	}
	if t.Missed != "" && t.Missed != MissedCatchUpOnce && t.Missed != MissedSkip {
		return fmt.Errorf("unknown missed policy %q", t.Missed)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDefinition, fmt.Sprintf(format, args...))
}
