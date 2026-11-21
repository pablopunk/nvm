// Package perf measures a running process tree from /proc and reports the counters the rig reads to
// check the budgets in PLAN.md section 3.2.
//
// The readers take an fs.FS rooted at a proc directory, so tests supply fake /proc content. Live
// returns the real one and reports ErrUnsupported where there is no Linux /proc. Compare evaluates a
// Report against a budget file and a stored baseline; cmd/perfcompare is its command.
package perf
