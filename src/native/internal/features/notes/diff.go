package notes

import "strings"

// maxDiffCells bounds the line-by-line comparison. Beyond it the middle of the two texts is shown as removed and
// then added, which is still correct and keeps a 512 KiB note from costing a gigabyte of table.
const maxDiffCells = 4_000_000

// DiffOp says what happened to a line between two versions.
type DiffOp int

// Diff operations: a line kept, added in the new version, or removed from the old one.
const (
	Keep DiffOp = iota
	Add
	Remove
)

// DiffLine is one line of a diff, with the text it carries.
type DiffLine struct {
	Op   DiffOp
	Text string
}

// DiffSummary counts the lines a diff adds and removes.
type DiffSummary struct {
	Added   int
	Removed int
}

// Diff compares two texts line by line. It returns the lines in order, each marked kept, added or removed; the
// lines the two texts share are kept, found by longest common subsequence after the common head and tail are
// set aside.
func Diff(before, after string) []DiffLine {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	out := make([]DiffLine, 0, len(a)+len(b))
	for _, line := range a[:head] {
		out = append(out, DiffLine{Op: Keep, Text: line})
	}
	out = append(out, middleDiff(a[head:len(a)-tail], b[head:len(b)-tail])...)
	for _, line := range a[len(a)-tail:] {
		out = append(out, DiffLine{Op: Keep, Text: line})
	}
	return out
}

// Summarize counts the added and removed lines of a diff.
func Summarize(lines []DiffLine) DiffSummary {
	var summary DiffSummary
	for _, line := range lines {
		switch line.Op {
		case Add:
			summary.Added++
		case Remove:
			summary.Removed++
		}
	}
	return summary
}

// middleDiff compares the lines between the shared head and tail.
func middleDiff(a, b []string) []DiffLine {
	if len(a)*len(b) > maxDiffCells {
		out := make([]DiffLine, 0, len(a)+len(b))
		for _, line := range a {
			out = append(out, DiffLine{Op: Remove, Text: line})
		}
		for _, line := range b {
			out = append(out, DiffLine{Op: Add, Text: line})
		}
		return out
	}
	// table[i][j] is the length of the common subsequence of a[i:] and b[j:].
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	var out []DiffLine
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, DiffLine{Op: Keep, Text: a[i]})
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, DiffLine{Op: Remove, Text: a[i]})
			i++
		default:
			out = append(out, DiffLine{Op: Add, Text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, DiffLine{Op: Remove, Text: a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, DiffLine{Op: Add, Text: b[j]})
	}
	return out
}
