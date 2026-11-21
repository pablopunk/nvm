package extview

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// stepAliases maps the status words extensions send to the three states a step shows, as the Electron view does.
var stepAliases = map[string]api.ProgressStatus{
	"done": api.StepComplete, "complete": api.StepComplete, "completed": api.StepComplete, "success": api.StepComplete,
	"active": api.StepActive, "running": api.StepActive, "loading": api.StepActive, "in progress": api.StepActive, "progress": api.StepActive,
	"error": api.StepFailed, "failed": api.StepFailed, "failure": api.StepFailed,
}

// normalizeStep is the state a step shows. Unknown or missing statuses are pending.
func normalizeStep(status api.ProgressStatus) api.ProgressStatus {
	if state, ok := stepAliases[string(status)]; ok {
		return state
	}
	if status == api.StepComplete || status == api.StepActive || status == api.StepFailed {
		return status
	}
	return api.StepPending
}

// progressPage shows a step or determinate progress view. Its body is read-only.
func (s *Session) progressPage() *palette.View {
	return &palette.View{
		Title: s.view.Title, HideInput: true,
		Body: func(c *ui.Context, _ search.Host) { drawProgress(c, s.view.Progress) },
	}
}

func drawProgress(c *ui.Context, spec *api.ProgressSpec) {
	if spec == nil {
		return
	}
	t := design.Tokens(c)
	ui.Column(c).Gap(6).Padding(14).Children(func() {
		ui.Text(c, progressSummary(spec)).FontSize(13).TextColor(t.Strong)
		if line := progressAmount(spec); line != "" {
			ui.Text(c, line).FontSize(11).TextColor(t.Tertiary)
		}
		for _, step := range spec.Steps {
			state := normalizeStep(step.Status)
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, stepMarker(state)).FontSize(11).TextColor(stepColor(t, state))
				ui.Text(c, step.Title).FontSize(11).TextColor(t.Primary)
			})
		}
	})
}

// progressSummary is the label, else the status, else "Working…", as the Electron view shows it.
func progressSummary(spec *api.ProgressSpec) string {
	return orDefault(spec.Label, orDefault(spec.Status, "Working…"))
}

// progressAmount names how far along the work is: the explicit value of a determinate bar, else the count of done steps.
func progressAmount(spec *api.ProgressSpec) string {
	total, value := spec.Total, spec.Value
	if total <= 0 {
		total = len(spec.Steps)
		value = 0
		for _, step := range spec.Steps {
			if normalizeStep(step.Status) == api.StepComplete {
				value++
			}
		}
	}
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d · %d%%", value, total, value*100/total)
}

func stepMarker(state api.ProgressStatus) string {
	switch state {
	case api.StepComplete:
		return "✓"
	case api.StepActive:
		return "●"
	case api.StepFailed:
		return "✕"
	}
	return "○"
}

func stepColor(t *design.TokenSet, state api.ProgressStatus) ui.Color {
	switch state {
	case api.StepComplete:
		return t.Strong
	case api.StepActive:
		return t.AccentEmphasis
	case api.StepFailed:
		return t.Danger
	}
	return t.Tertiary
}
