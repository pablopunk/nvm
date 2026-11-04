package builder

import (
	"errors"
	"strings"
	"testing"
)

func TestParseReadsTheLastBlocks(t *testing.T) {
	text := "First try:\n```json\n{\"apiVersion\":\"1.0\",\"id\":\"user.old\",\"title\":\"Old\"}\n```\n```go\npackage main\n```\n" +
		"Corrected:\n```json\n{\"apiVersion\":\"1.0\",\"id\":\"user.clock\",\"title\":\"Clock\"}\n```\n```go\npackage main\n// fixed\n```\n"

	candidate, err := ParseReply(text)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Manifest.ID != "user.clock" || !strings.Contains(candidate.Source, "// fixed") {
		t.Fatalf("parsed %+v; want the last manifest and source", candidate)
	}
}

func TestParseRefusesMissingBlocks(t *testing.T) {
	if _, err := ParseReply("Only prose."); !errors.Is(err, ErrNoBlocks) {
		t.Fatalf("err = %v; want ErrNoBlocks", err)
	}
}

func TestParseRefusesAManifestTheAPIRejects(t *testing.T) {
	cases := map[string]string{
		"not json":       "```json\n{nope\n```\n```go\npackage main\n```",
		"bad id":         "```json\n{\"apiVersion\":\"1.0\",\"id\":\"Clock!\",\"title\":\"Clock\"}\n```\n```go\npackage main\n```",
		"reserved id":    "```json\n{\"apiVersion\":\"1.0\",\"id\":\"nevermind.clock\",\"title\":\"Clock\"}\n```\n```go\npackage main\n```",
		"missing title":  "```json\n{\"apiVersion\":\"1.0\",\"id\":\"user.clock\"}\n```\n```go\npackage main\n```",
		"future version": "```json\n{\"apiVersion\":\"9.0\",\"id\":\"user.clock\",\"title\":\"Clock\"}\n```\n```go\npackage main\n```",
	}
	for name, text := range cases {
		if _, err := ParseReply(text); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}
