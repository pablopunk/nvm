package notes

import "strings"

// maxDiffCells bounds the line-by-line comparison. Beyond it the middle of the two texts is shown as removed and then
// added, which is still correct and keeps a 512 KiB note from costing a gigabyte of table.
const maxDiffCells = 4_000_000

// diffOp says what happened to a line between two versions.
type diffOp int

// Diff operations: a line kept, added in the new version, or removed from the old one.
const (
	opKeep diffOp = iota
	opAdd
	opRemove
)

// diffLine is one line of a diff, with the text it carries.
type diffLine struct {
	Op   diffOp
	Text string
}

// diffLines compares two texts line by line. The lines the two texts share are kept, found by longest common
// subsequence after the common head and tail are set aside.
func diffLines(before, after string) []diffLine {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	out := make([]diffLine, 0, len(a)+len(b))
	for _, line := range a[:head] {
		out = append(out, diffLine{Op: opKeep, Text: line})
	}
	out = append(out, middleDiff(a[head:len(a)-tail], b[head:len(b)-tail])...)
	for _, line := range a[len(a)-tail:] {
		out = append(out, diffLine{Op: opKeep, Text: line})
	}
	return out
}

// diffMark is the prefix a diff line is drawn with.
func diffMark(op diffOp) string {
	switch op {
	case opAdd:
		return "+ "
	case opRemove:
		return "- "
	}
	return "  "
}

// middleDiff compares the lines between the shared head and tail.
func middleDiff(a, b []string) []diffLine {
	if len(a)*len(b) > maxDiffCells {
		out := make([]diffLine, 0, len(a)+len(b))
		for _, line := range a {
			out = append(out, diffLine{Op: opRemove, Text: line})
		}
		for _, line := range b {
			out = append(out, diffLine{Op: opAdd, Text: line})
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
	var out []diffLine
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, diffLine{Op: opKeep, Text: a[i]})
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, diffLine{Op: opRemove, Text: a[i]})
			i++
		default:
			out = append(out, diffLine{Op: opAdd, Text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, diffLine{Op: opRemove, Text: a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, diffLine{Op: opAdd, Text: b[j]})
	}
	return out
}
