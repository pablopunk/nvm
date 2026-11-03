package perf

import (
	"bytes"
	"strings"
	"testing"
)

func testBudgets() BudgetFile {
	return BudgetFile{
		Proposed: true,
		Budgets: []Budget{
			{ID: "B1", Metric: "B1.p95_ms", Max: 150, Unit: "ms", Proposed: true},
			{ID: "B5", Metric: "B5.cpu_pct", Op: "<", Max: 1, Unit: "%", Proposed: true},
			{ID: "B6", Metric: "B6.binary_mib", Max: 60, Unit: "MiB", Proposed: true},
			{ID: "B9", Metric: "B9.absent", Max: 1, Unit: "x", Proposed: true},
		},
	}
}

func testReport(values map[string]float64) Report {
	report := Report{Environment: Environment{Label: "test", Renderer: "llvmpipe", CPUs: 4}, Measurements: map[string]Measurement{}}
	for metric, value := range values {
		report.Measurements[metric] = Measurement{Value: value, Unit: "ms"}
	}
	return report
}

func TestCompareAppliesLimitsAndOperators(t *testing.T) {
	report := testReport(map[string]float64{"B1.p95_ms": 150, "B5.cpu_pct": 1, "B6.binary_mib": 61})
	rows := Compare(testBudgets(), report, nil)
	want := map[string]bool{"B1": true, "B5": false, "B6": false, "B9": false}
	for _, row := range rows {
		if row.Pass != want[row.Budget.ID] {
			t.Errorf("%s pass = %v, want %v (value %v, found %v)", row.Budget.ID, row.Pass, want[row.Budget.ID], row.Value, row.Found)
		}
	}
	if got := Failures(rows); got != 3 {
		t.Fatalf("Failures = %d, want 3 (B5 is strict, B6 over, B9 missing)", got)
	}
}

func TestCompareMarksMissingMetricsAsFailures(t *testing.T) {
	rows := Compare(testBudgets(), testReport(nil), nil)
	for _, row := range rows {
		if row.Found || row.Pass {
			t.Fatalf("%s found=%v pass=%v with an empty report", row.Budget.ID, row.Found, row.Pass)
		}
	}
}

func TestCompareReadsBaselineDeltas(t *testing.T) {
	baseline := testReport(map[string]float64{"B1.p95_ms": 100})
	report := testReport(map[string]float64{"B1.p95_ms": 120})
	rows := Compare(testBudgets(), report, &baseline)
	if !rows[0].HasBaseline || rows[0].Baseline != 100 {
		t.Fatalf("baseline row = %+v", rows[0])
	}
	var out bytes.Buffer
	if err := WriteTable(&out, report, rows); err != nil {
		t.Fatal(err)
	}
	table := out.String()
	for _, want := range []string{"B1", "+20.00 ms", "MISSING", "environment: test, renderer llvmpipe, 4 CPUs"} {
		if !strings.Contains(table, want) {
			t.Errorf("table lacks %q:\n%s", want, table)
		}
	}
}
