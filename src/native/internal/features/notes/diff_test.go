package notes

import (
	"fmt"
	"strings"
	"testing"
)

func lines(ops ...DiffLine) []DiffLine { return ops }

func keep(s string) DiffLine   { return DiffLine{Op: Keep, Text: s} }
func add(s string) DiffLine    { return DiffLine{Op: Add, Text: s} }
func remove(s string) DiffLine { return DiffLine{Op: Remove, Text: s} }

func TestDiffMarksKeptAddedAndRemovedLines(t *testing.T) {
	cases := []struct {
		name, before, after string
		want                []DiffLine
	}{
		{"identical", "a\nb", "a\nb", lines(keep("a"), keep("b"))},
		{"replaced middle line", "a\nb\nc", "a\nx\nc", lines(keep("a"), remove("b"), add("x"), keep("c"))},
		{"inserted line", "a\nc", "a\nb\nc", lines(keep("a"), add("b"), keep("c"))},
		{"deleted line", "a\nb\nc", "a\nc", lines(keep("a"), remove("b"), keep("c"))},
		{"empty to text", "", "new", lines(remove(""), add("new"))},
		{"text to empty", "old", "", lines(remove("old"), add(""))},
		{"reordered lines", "a\nb", "b\na", lines(remove("a"), keep("b"), add("a"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Diff(tc.before, tc.after)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("Diff(%q, %q) = %v, want %v", tc.before, tc.after, got, tc.want)
			}
		})
	}
}

func TestSummarizeCountsAddedAndRemovedLines(t *testing.T) {
	summary := Summarize(Diff("one\ntwo\nthree", "one\n2\n3\nfour"))
	mustEqual(t, "added", summary.Added, 3)
	mustEqual(t, "removed", summary.Removed, 2)
	mustEqual(t, "identical added", Summarize(Diff("same", "same")).Added, 0)
}

func TestDiffOfLargeTextsFallsBackWithinTheCellBudget(t *testing.T) {
	var before, after []string
	for i := range 3000 {
		before = append(before, fmt.Sprintf("old %d", i))
		after = append(after, fmt.Sprintf("new %d", i))
	}
	got := Diff(strings.Join(before, "\n"), strings.Join(after, "\n"))
	summary := Summarize(got)
	mustEqual(t, "added", summary.Added, 3000)
	mustEqual(t, "removed", summary.Removed, 3000)
	mustEqual(t, "lines", len(got), 6000)
}
