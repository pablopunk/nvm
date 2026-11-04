package extview

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestProgressStatusWordsNormalizeToThreeStates(t *testing.T) {
	tests := map[api.ProgressStatus]api.ProgressStatus{
		"success": api.StepComplete, "completed": api.StepComplete, "running": api.StepActive,
		"in progress": api.StepActive, "failure": api.StepFailed, "": api.StepPending, "weird": api.StepPending,
	}
	for in, want := range tests {
		if got := normalizeStep(in); got != want {
			t.Fatalf("normalizeStep(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProgressAmountUsesTheValueOrTheDoneSteps(t *testing.T) {
	determinate := &api.ProgressSpec{Value: 1, Total: 4}
	if got := progressAmount(determinate); got != "1 of 4 · 25%" {
		t.Fatalf("determinate = %q", got)
	}
	steps := &api.ProgressSpec{Steps: []api.ProgressStep{
		{Title: "a", Status: api.StepComplete}, {Title: "b", Status: api.StepActive}, {Title: "c"},
	}}
	if got := progressAmount(steps); got != "1 of 3 · 33%" {
		t.Fatalf("steps = %q", got)
	}
	if got := progressSummary(&api.ProgressSpec{}); got != "Working…" {
		t.Fatalf("summary of an empty spec = %q", got)
	}
}

func TestTextPreviewFenceIsLongerThanAnyBacktickRun(t *testing.T) {
	got := textPreviewMarkdown("a ``` b", ".go", 7)
	if !strings.HasPrefix(got, "````go\n") || !strings.HasSuffix(got, "\n````") {
		t.Fatalf("fence = %q", got)
	}
}

func TestTextPreviewNotesTruncation(t *testing.T) {
	got := textPreviewMarkdown("hello", ".txt", 200<<10)
	if !strings.Contains(got, "_Showing the first 0 KB of 200 KB._") {
		t.Fatalf("truncation note missing: %q", got)
	}
}

func TestFilePreviewReadsTextAndNamesBinaryFiles(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/notes", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/notes/a.txt", []byte("first line\n"), fs.FileMode(0o644)); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/notes/b.bin", []byte{0, 1, 2, 3}, fs.FileMode(0o644)); err != nil {
		t.Fatal(err)
	}
	s := NewSession(Deps{Files: files}, "acme", api.View{Kind: api.ViewFilePreview, Title: "Preview"})
	if got := s.fileMarkdown(api.FileRef{Path: "/notes/a.txt", Extension: ".txt"}); !strings.Contains(got, "first line") {
		t.Fatalf("text file preview = %q", got)
	}
	if got := s.fileMarkdown(api.FileRef{Path: "/notes/b.bin"}); !strings.Contains(got, "Binary file") {
		t.Fatalf("binary file preview = %q", got)
	}
	if got := s.fileMarkdown(api.FileRef{Path: "/notes/missing"}); !strings.Contains(got, "Could not read this file") {
		t.Fatalf("missing file preview = %q", got)
	}
}
