package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBackoffDoublesFromItsInitialDelayUpToItsCap(t *testing.T) {
	backoff := Backoff{}
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second, 480 * time.Second, 900 * time.Second, 900 * time.Second}
	for i, delay := range want {
		if got := backoff.delayAfter(i + 1); got != delay {
			t.Fatalf("after failure %d: %v, want %v", i+1, got, delay)
		}
	}
	custom := Backoff{Initial: time.Second, Max: 3 * time.Second}
	if custom.delayAfter(1) != time.Second || custom.delayAfter(2) != 2*time.Second || custom.delayAfter(5) != 3*time.Second {
		t.Fatal("custom backoff does not follow its initial delay and cap")
	}
}

func TestHaltPolicyDefaultsAreThreeFailuresInAMinute(t *testing.T) {
	failures, within := HaltPolicy{}.limits()
	if failures != 3 || within != time.Minute {
		t.Fatalf("defaults = %d within %v", failures, within)
	}
}

func TestPruneKeepsOnlyTimesInsideTheWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	times := []time.Time{now.Add(-2 * time.Minute), now.Add(-30 * time.Second), now}
	kept := prune(times, now, time.Minute)
	if len(kept) != 2 || !kept[0].Equal(now.Add(-30*time.Second)) {
		t.Fatalf("kept = %v", kept)
	}
}

func TestDebounceWaitsForQuietAndIsCutShortByMaxWait(t *testing.T) {
	start := time.Unix(0, 0)
	trigger := Trigger{Kind: TriggerEvent, Event: "e", Debounce: 2 * time.Second, MaxWait: 5 * time.Second}
	cases := []struct {
		now  time.Duration
		want time.Duration
	}{
		{0, 2 * time.Second},
		{3 * time.Second, 2 * time.Second},
		{4 * time.Second, 1 * time.Second},
		{6 * time.Second, 0},
	}
	for _, tc := range cases {
		if got := debounceDelay(trigger, start, start.Add(tc.now)); got != tc.want {
			t.Fatalf("at %v: delay %v, want %v", tc.now, got, tc.want)
		}
	}
	if got := debounceDelay(Trigger{Kind: TriggerEvent, Event: "e", Debounce: time.Second}, start, start.Add(time.Hour)); got != time.Second {
		t.Fatalf("no max wait: delay %v", got)
	}
}

func TestMissedDueOnlyWhenAFullPeriodPassedAndTheTriggerCatchesUp(t *testing.T) {
	last := time.Unix(0, 0)
	hourly := Trigger{Kind: TriggerInterval, Every: time.Hour}
	if missedDue(hourly, time.Time{}, last.Add(2*time.Hour)) {
		t.Fatal("a job that never ran counted as missed")
	}
	if missedDue(hourly, last, last.Add(time.Hour-time.Second)) {
		t.Fatal("a run not yet due counted as missed")
	}
	if !missedDue(hourly, last, last.Add(time.Hour)) {
		t.Fatal("a due run was not missed")
	}
	if missedDue(Trigger{Kind: TriggerInterval, Every: time.Hour, Missed: MissedSkip}, last, last.Add(2*time.Hour)) {
		t.Fatal("skip policy caught up")
	}
}

func TestMergePayloadsUnionsChangedPathsNewestLast(t *testing.T) {
	first := Payload{"changedPaths": []string{"/a", "/b"}, "trigger": "x"}
	second := Payload{"changedPaths": []string{"/b", "/c"}, "trigger": "y"}
	merged := mergePayloads(first, second)
	if got := merged["changedPaths"].([]string); len(got) != 3 || got[2] != "/c" {
		t.Fatalf("changedPaths = %v", got)
	}
	if merged["trigger"] != "y" {
		t.Fatalf("later key did not win: %v", merged["trigger"])
	}
	if got := first["changedPaths"].([]string); len(got) != 2 {
		t.Fatal("merge modified its input")
	}
	if mergePayloads(nil, second)["trigger"] != "y" || mergePayloads(first, nil)["trigger"] != "x" {
		t.Fatal("merging with nil lost a side")
	}
}

func TestMergedChangedPathsAreBoundedToTheNewest(t *testing.T) {
	var paths []string
	for i := range maxChangedPaths + 10 {
		paths = append(paths, string(rune('A'+i%26))+string(rune('a'+i/26)))
	}
	merged := mergePayloads(Payload{"changedPaths": paths[:50]}, Payload{"changedPaths": paths[50:]})
	got := merged["changedPaths"].([]string)
	if len(got) != maxChangedPaths || got[len(got)-1] != paths[len(paths)-1] {
		t.Fatalf("bounded paths = %d, last %q", len(got), got[len(got)-1])
	}
}

func TestInvalidDefinitionsAreRefusedWithTheInvalidDefinitionError(t *testing.T) {
	bad := map[string]Definition{
		"no id":             {Title: "T", Run: nil},
		"no title":          {ID: "a", Run: fnRun},
		"no run":            {ID: "a", Title: "T"},
		"unknown owner":     {ID: "a", Title: "T", Owner: "robot", Run: fnRun},
		"negative timeout":  {ID: "a", Title: "T", Run: fnRun, Budget: Budget{Timeout: -1}},
		"runs without":      {ID: "a", Title: "T", Run: fnRun, Budget: Budget{MaxRuns: 2}},
		"fast interval":     {ID: "a", Title: "T", Run: fnRun, Triggers: []Trigger{{Kind: TriggerInterval, Every: time.Millisecond}}},
		"startup with time": {ID: "a", Title: "T", Run: fnRun, Triggers: []Trigger{{Kind: TriggerStartup, Every: time.Second}}},
		"event no name":     {ID: "a", Title: "T", Run: fnRun, Triggers: []Trigger{{Kind: TriggerEvent}}},
		"unknown kind":      {ID: "a", Title: "T", Run: fnRun, Triggers: []Trigger{{Kind: "cron"}}},
		"unknown missed":    {ID: "a", Title: "T", Run: fnRun, Triggers: []Trigger{{Kind: TriggerInterval, Every: time.Second, Missed: "always"}}},
	}
	for name, def := range bad {
		if err := def.validate(); !errors.Is(err, ErrInvalidDefinition) {
			t.Errorf("%s: err = %v, want ErrInvalidDefinition", name, err)
		}
	}
	valid := Definition{ID: "a", Title: "T", Run: fnRun, Budget: Budget{MaxRuns: 1, Window: time.Minute}, Triggers: []Trigger{{Kind: TriggerInterval, Every: time.Second}}}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid definition refused: %v", err)
	}
}

func fnRun(_ context.Context, _ Run) error { return nil }
