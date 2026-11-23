package updates

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// rowRootUpdate is the id of the root's update row.
const rowRootUpdate = "update-root"

// rootUpdateScore is the update row's score, as 1.x's update prompt has it: above every other root row.
const rootUpdateScore = 1000

// RootItems shows an update that is found, downloading or ready at the root, above the commands, where the person is
// about to search. A failed install shows too, so a refused update is never silent; a failed check does not, because
// an offline laptop would show it at every launch.
func (Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	return rootRows(ctx.Updates().State()), nil
}

// SearchItems keeps the update row while the query names updates, and leaves other queries alone.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !queryNamesUpdates(query) {
		return nil, nil
	}
	return rootRows(ctx.Updates().State()), nil
}

// rootRows is the update row for a state, or none while there is no update to show.
func rootRows(state api.UpdateState) []api.Item {
	if !showsUpdate(state) {
		return nil
	}
	row := api.Item{
		ID: rowRootUpdate, Title: state.Message, Subtitle: "Nevermind updates", Icon: "arrow-down",
		Section: search.KindCommands, LearnKey: "updates", Score: rootUpdateScore,
		Busy:    state.Status == api.UpdateStatusAvailable || state.Status == api.UpdateStatusDownloading,
		Actions: []api.Action{api.Run(openAction, "Open Updates", nil)},
	}
	switch state.Status {
	case api.UpdateStatusDownloaded:
		row.Icon = "rotate-ccw"
		row.Appearance = &api.Appearance{Background: "accent"}
		row.Actions = []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpInstall, Title: "Restart to Update", DismissAfterRun: true}}
	case api.UpdateStatusError:
		row.Icon = "circle-alert"
	}
	return []api.Item{row}
}

// showsUpdate is whether the root lists the update row for a state.
func showsUpdate(state api.UpdateState) bool {
	switch state.Status {
	case api.UpdateStatusAvailable, api.UpdateStatusDownloading, api.UpdateStatusDownloaded:
		return true
	case api.UpdateStatusError:
		return state.FailedStage == api.UpdateStageInstall
	}
	return false
}

// queryNamesUpdates reports whether a query that starts the word update, or names a Nevermind update, keeps the row.
func queryNamesUpdates(q string) bool {
	lq := strings.ToLower(strings.TrimSpace(q))
	return lq == "" || strings.HasPrefix("update", lq) || strings.Contains("nevermind update", lq)
}
