package perf

import (
	"math"
	"slices"
)

// Report is what the rig writes to perf-report.json: one run's environment and measurements.
type Report struct {
	Environment  Environment            `json:"environment"`
	Measurements map[string]Measurement `json:"measurements"`
}

// Environment says where a run was measured. A number is only comparable with numbers from the same
// environment label, renderer and CPU count.
type Environment struct {
	Label     string `json:"label"`
	Renderer  string `json:"renderer"`
	CPUs      int    `json:"cpus"`
	Kernel    string `json:"kernel"`
	GoVersion string `json:"goVersion"`
	Display   string `json:"display"`
	Recorded  string `json:"recorded"`
}

// Measurement is one number from a run. Samples holds the raw values behind a percentile.
type Measurement struct {
	Value   float64   `json:"value"`
	Unit    string    `json:"unit"`
	Samples []float64 `json:"samples,omitempty"`
	Note    string    `json:"note,omitempty"`
}

// Percentile returns the nearest-rank p-th percentile (0 to 100) of samples. It does not modify
// samples, and returns 0 for no samples.
func Percentile(samples []float64, p float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := sortedCopy(samples)
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	rank = max(1, min(rank, len(sorted)))
	return sorted[rank-1]
}

// Median returns the middle value of samples, or 0 for none. Samples are not modified.
func Median(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := sortedCopy(samples)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func sortedCopy(samples []float64) []float64 {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	return sorted
}
