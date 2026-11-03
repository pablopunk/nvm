package perf

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// BudgetFile is testdata/budgets.json: the thresholds from PLAN.md section 3.2. Proposed budgets stay
// proposed until the owner ratifies them.
type BudgetFile struct {
	Proposed bool     `json:"proposed"`
	Ratified bool     `json:"ratified"`
	Source   string   `json:"source"`
	Budgets  []Budget `json:"budgets"`
}

// Budget is one threshold on one measurement. Op is "<=" (the default) or "<".
type Budget struct {
	ID       string  `json:"id"`
	Metric   string  `json:"metric"`
	Op       string  `json:"op,omitempty"`
	Max      float64 `json:"max"`
	Unit     string  `json:"unit"`
	Text     string  `json:"text"`
	Proposed bool    `json:"proposed"`
}

// Row is one budget evaluated against a run and, when the baseline has the metric, against it.
type Row struct {
	Budget      Budget
	Value       float64
	Found       bool
	Pass        bool
	Baseline    float64
	HasBaseline bool
}

// Compare evaluates every budget against report, and against baseline where it has the metric. A
// budget whose metric is missing from the report fails.
func Compare(budgets BudgetFile, report Report, baseline *Report) []Row {
	rows := make([]Row, 0, len(budgets.Budgets))
	for _, budget := range budgets.Budgets {
		row := Row{Budget: budget}
		if m, ok := report.Measurements[budget.Metric]; ok {
			row.Value, row.Found = m.Value, true
			row.Pass = withinBudget(m.Value, budget)
		}
		if baseline != nil {
			if m, ok := baseline.Measurements[budget.Metric]; ok {
				row.Baseline, row.HasBaseline = m.Value, true
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// Failures counts the rows that do not pass.
func Failures(rows []Row) int {
	n := 0
	for _, row := range rows {
		if !row.Pass {
			n++
		}
	}
	return n
}

func withinBudget(value float64, budget Budget) bool {
	if budget.Op == "<" {
		return value < budget.Max
	}
	return value <= budget.Max
}

// WriteTable prints the rows as a table with the status of each budget and the delta from the
// baseline. The environment line says what the numbers were measured on.
func WriteTable(w io.Writer, report Report, rows []Row) error {
	env := report.Environment
	if _, err := fmt.Fprintf(w, "environment: %s, renderer %s, %d CPUs, kernel %s, %s, display %s\n\n",
		env.Label, env.Renderer, env.CPUs, env.Kernel, env.GoVersion, env.Display); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "BUDGET\tMETRIC\tVALUE\tLIMIT\tSTATUS\tBASELINE\tDELTA")
	for _, row := range rows {
		status := "PASS"
		value, limit := "missing", fmt.Sprintf("%s %g %s", opOf(row.Budget), row.Budget.Max, row.Budget.Unit)
		baseline, delta := "-", "-"
		if row.Found {
			value = fmt.Sprintf("%.2f %s", row.Value, row.Budget.Unit)
		} else {
			status = "MISSING"
		}
		if !row.Pass && row.Found {
			status = "FAIL"
		}
		if row.HasBaseline {
			baseline = fmt.Sprintf("%.2f %s", row.Baseline, row.Budget.Unit)
			if row.Found {
				delta = fmt.Sprintf("%+.2f %s", row.Value-row.Baseline, row.Budget.Unit)
			}
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.Budget.ID, row.Budget.Metric, value, limit, status, baseline, delta)
	}
	return table.Flush()
}

func opOf(budget Budget) string {
	if budget.Op == "<" {
		return "<"
	}
	return "<="
}
