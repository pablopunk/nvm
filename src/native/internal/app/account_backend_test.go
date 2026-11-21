package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// reportOf is a compatibility report whose backend runs in environment at version.
func reportOf(environment, version string) *backend.CompatReport {
	r := &backend.CompatReport{State: backend.CompatOK}
	r.Manifest.Backend = backend.BackendInfo{Environment: environment, Version: version}
	return r
}

// TestBackendStatusRowWordsItAsOneX pins the status row: the environment and its origin, the server's environment and
// version or unavailable, and the mismatch mark when the server runs another environment.
func TestBackendStatusRowWordsItAsOneX(t *testing.T) {
	cases := []struct {
		name, title, icon, subtitle string
		active                      backend.EnvironmentKind
		report                      *backend.CompatReport
	}{
		{"no report", "Nevermind: Backend Status", "globe",
			"Client production · Active production · https://api.nvm.fyi · Server unavailable",
			backend.EnvironmentProduction, nil},
		{"same environment", "Nevermind: Backend Status", "globe",
			"Client production · Active production · https://api.nvm.fyi · Server production (2.4.0)",
			backend.EnvironmentProduction, reportOf("production", "2.4.0")},
		{"preview is named preview", "Nevermind: Backend Status", "globe",
			"Client pr_preview · Active pr_preview · https://nvm-x.vercel.app · Server preview (2.4.0)",
			backend.EnvironmentPRPreview, reportOf("preview", "2.4.0")},
		{"another environment", "Nevermind: Backend Status — mismatch", "circle-alert",
			"Client development · Active development · http://localhost:4321 · Server production (2.4.0)",
			backend.EnvironmentDevelopment, reportOf("production", "2.4.0")},
	}
	for _, c := range cases {
		url := map[backend.EnvironmentKind]string{backend.EnvironmentProduction: "https://api.nvm.fyi",
			backend.EnvironmentPRPreview: "https://nvm-x.vercel.app", backend.EnvironmentDevelopment: "http://localhost:4321"}[c.active]
		title, subtitle, icon := backendStatusRow(c.active, url, c.report)
		if title != c.title || subtitle != c.subtitle || icon != c.icon {
			t.Fatalf("%s: row = %q / %q / %q, want %q / %q / %q", c.name, title, subtitle, icon, c.title, c.subtitle, c.icon)
		}
	}
}

// TestEnvironmentChoicesFollowThePackagedBuild pins the switch's rows in 1.x's order, and that Development is offered
// only in an unpackaged build. The preview and custom rows ask for their origin.
func TestEnvironmentChoicesFollowThePackagedBuild(t *testing.T) {
	var titles []string
	for _, c := range environmentChoices(false) {
		titles = append(titles, c.title)
	}
	if got := strings.Join(titles, "|"); got != "Development|Preview URL…|Production|Custom URL…" {
		t.Fatalf("unpackaged choices = %s", got)
	}
	for _, c := range environmentChoices(true) {
		if c.kind == backend.EnvironmentDevelopment {
			t.Fatal("a packaged build offers Development")
		}
	}
	if fields := environmentChoices(true); fields[0].title != "Preview URL…" || fields[0].fieldLabel == "" {
		t.Fatalf("the preview choice does not ask for its URL: %+v", fields[0])
	}
}

// TestRootEmptyListsTheBackendRowsAtTheirOneXScores pins the two backend rows of the empty root at 1.x's scores, each
// listed once. The rows are read before the 30-row cap, which a low score can fall under.
func TestRootEmptyListsTheBackendRowsAtTheirOneXScores(t *testing.T) {
	rows := commandRows(goldenPalette(t), &rootInput{}, "")
	env := findRowTitled(t, rows, "Nevermind: Switch Backend Environment")
	status := findRowTitled(t, rows, "Nevermind: Backend Status")
	if env.Score != backendEnvironmentRootScore || status.Score != backendStatusRootScore {
		t.Fatalf("backend rows score %d and %d, want %d and %d", env.Score, status.Score, backendEnvironmentRootScore, backendStatusRootScore)
	}
}
