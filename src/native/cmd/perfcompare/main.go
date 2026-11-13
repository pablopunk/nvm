// Command perfcompare checks a rig perf report against the budgets, prints the table with the deltas
// from the stored baseline, and exits 1 on a budget violation. It is the comparison step of
// scripts/rig/perf.sh; -warn-only prints the same table and exits 0.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/pablopunk/nvm/native/internal/perf"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("perfcompare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	budgetsPath := flags.String("budgets", "testdata/budgets.json", "budget file")
	reportPath := flags.String("report", "", "perf-report.json written by the rig run (required)")
	baselinePath := flags.String("baseline", "testdata/perf-baseline.json", "stored baseline; a missing file means there is none")
	warnOnly := flags.Bool("warn-only", false, "print budget violations as warnings and exit 0")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reportPath == "" {
		fmt.Fprintln(stderr, "perfcompare: -report is required")
		return 2
	}

	var budgets perf.BudgetFile
	if err := readJSON(*budgetsPath, &budgets); err != nil {
		fmt.Fprintf(stderr, "perfcompare: budgets: %v\n", err)
		return 2
	}
	var report perf.Report
	if err := readJSON(*reportPath, &report); err != nil {
		fmt.Fprintf(stderr, "perfcompare: report: %v\n", err)
		return 2
	}
	baseline, err := readBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "perfcompare: baseline: %v\n", err)
		return 2
	}

	rows := perf.Compare(budgets, report, baseline)
	if err := perf.WriteTable(stdout, report, rows); err != nil {
		fmt.Fprintf(stderr, "perfcompare: print: %v\n", err)
		return 2
	}
	failures := perf.Failures(rows)
	if failures == 0 {
		fmt.Fprintln(stdout, "\nall budgets pass")
		return 0
	}
	if *warnOnly {
		fmt.Fprintf(stdout, "\nWARN: %d budget violation(s); not failing (-warn-only)\n", failures)
		return 0
	}
	fmt.Fprintf(stderr, "perfcompare: %d budget violation(s)\n", failures)
	return 1
}

func readBaseline(path string) (*perf.Report, error) {
	var baseline perf.Report
	err := readJSON(path, &baseline)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &baseline, nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
