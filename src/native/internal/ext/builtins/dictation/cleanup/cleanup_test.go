package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func answering(text string, err error) Completer {
	return func(context.Context, string, string) (string, error) { return text, err }
}

func TestCleanReturnsTheModelsTrimmedAnswer(t *testing.T) {
	got := Clean(context.Background(), answering("  Hello, world.\n", nil), "hello world", Terms{}, nil)
	if got != "Hello, world." {
		t.Fatalf("Clean = %q, want the trimmed answer", got)
	}
}

func TestCleanKeepsTheTranscriptWhenTheModelFailsOrAnswersNothing(t *testing.T) {
	var warned []error
	warn := func(err error) { warned = append(warned, err) }
	if got := Clean(context.Background(), answering("", errors.New("route paused")), "raw", Terms{}, warn); got != "raw" {
		t.Fatalf("Clean on failure = %q, want the transcript", got)
	}
	if got := Clean(context.Background(), answering("   ", nil), "raw", Terms{}, warn); got != "raw" {
		t.Fatalf("Clean on an empty answer = %q, want the transcript", got)
	}
	if len(warned) != 1 {
		t.Fatalf("warnings = %v, want the failure reported once", warned)
	}
}

func TestCleanStopsWaitingForAModelThatRunsPastTheDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	slow := func(context.Context, string, string) (string, error) {
		<-release
		return "late", nil
	}
	var warned error
	got := cleanWithin(context.Background(), time.Millisecond, slow, "raw", Terms{}, func(err error) { warned = err })
	if got != "raw" || !errors.Is(warned, ErrTimeout) {
		t.Fatalf("Clean past the deadline = %q, warned %v; want the transcript and the timeout", got, warned)
	}
}

func TestCleanSendsTheSystemInstructionAndThePromptWithTheTerms(t *testing.T) {
	var gotSystem, gotUser string
	capture := func(_ context.Context, system, user string) (string, error) {
		gotSystem, gotUser = system, user
		return "ok", nil
	}
	Clean(context.Background(), capture, "transcript body", NewTerms("Kubernetes", []string{"Grafana"}), nil)
	if gotSystem != System {
		t.Fatalf("system = %q, want the cleanup instruction", gotSystem)
	}
	if !strings.Contains(gotUser, "Preferred terms and spellings") || !strings.Contains(gotUser, "Grafana") {
		t.Fatalf("user prompt = %q, want the dictionary and screen terms", gotUser)
	}
	if !strings.HasSuffix(gotUser, "\nTranscript:\ntranscript body") {
		t.Fatalf("user prompt = %q, want the transcript last", gotUser)
	}
}
