package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJSON writes v into dir/name and returns the path.
func writeJSON(t *testing.T, dir, name string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureFiles(t *testing.T, latencyMS float64) (budgets, report string) {
	t.Helper()
	dir := t.TempDir()
	budgets = writeJSON(t, dir, "budgets.json", map[string]any{
		"proposed": true,
		"budgets": []map[string]any{
			{"id": "B2", "metric": "B2.p95_ms", "max": 16, "unit": "ms", "proposed": true},
		},
	})
	report = writeJSON(t, dir, "report.json", map[string]any{
		"environment": map[string]any{"label": "test"},
		"measurements": map[string]any{
			"B2.p95_ms": map[string]any{"value": latencyMS, "unit": "ms"},
		},
	})
	return budgets, report
}

func TestRunExitsOneOnBudgetViolation(t *testing.T) {
	budgets, report := fixtureFiles(t, 66)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-budgets", budgets, "-report", report, "-baseline", filepath.Join(t.TempDir(), "none.json")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FAIL") {
		t.Fatalf("table lacks FAIL:\n%s", stdout.String())
	}
}

func TestRunWarnOnlyDoesNotFail(t *testing.T) {
	budgets, report := fixtureFiles(t, 66)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-budgets", budgets, "-report", report, "-warn-only"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 with -warn-only", code)
	}
	if !strings.Contains(stdout.String(), "WARN") {
		t.Fatalf("output lacks WARN:\n%s", stdout.String())
	}
}

func TestRunPassesWithinBudget(t *testing.T) {
	budgets, report := fixtureFiles(t, 9)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-budgets", budgets, "-report", report}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0; stdout:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "all budgets pass") {
		t.Fatalf("output lacks the pass line:\n%s", stdout.String())
	}
}

func TestRunRequiresReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit without -report = %d, want 2", code)
	}
}
