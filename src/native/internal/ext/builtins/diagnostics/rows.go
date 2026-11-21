package diagnostics

import "github.com/pablopunk/nvm/native/internal/ext/api"

// referenceTimeLayout is how a row shows when its error was recorded, in UTC.
const referenceTimeLayout = "2006-01-02 15:04:05"

// Labels of the copy actions, as the native page named them.
const (
	copyReferenceTitle   = "Copy Error Reference"
	copyDiagnosticsTitle = "Copy Safe Diagnostics"
)

// recordRows is one row per recorded error, newest first, each with the reference as its primary action.
func recordRows(ctx api.Context) []api.Item {
	records := ctx.Diagnostics().Recent()
	label := reportingLabel(ctx.Diagnostics().Reporting())
	rows := make([]api.Item, 0, len(records))
	for _, record := range records {
		rows = append(rows, recordRow(ctx, record, label))
	}
	return rows
}

// recordRow shows a problem report as one, and any other record under its component. Its accessory says how the record is
// reported, and its actions copy the reference and the safe text.
func recordRow(ctx api.Context, record api.DiagnosticRecord, reporting string) api.Item {
	title, icon := record.Component, "circle-alert"
	if record.Component == api.DiagnosticsProblemComponent {
		title, icon = "Problem report", "message-circle"
	}
	copyReference := api.CopyText(copyReferenceTitle, record.Ref, false)
	actions := []api.Action{copyReference}
	if text, err := ctx.Diagnostics().Export(record.Ref); err == nil {
		actions = append(actions, api.CopyText(copyDiagnosticsTitle, text, false))
	}
	return api.Item{
		ID:          record.Ref,
		Title:       title,
		Subtitle:    record.Ref + " · " + record.Time.UTC().Format(referenceTimeLayout),
		Icon:        icon,
		Appearance:  &api.Appearance{Foreground: api.ColorNone},
		Accessories: []api.Accessory{{Text: reporting}},
		Primary:     api.Primary(copyReference),
		Actions:     actions,
	}
}

// reportingLabel says how a record is reported. Remote availability is never confirmed here: a sent event can still be
// lost on the way.
func reportingLabel(state string) string {
	switch state {
	case api.DiagnosticsOff:
		return "Not reported"
	case api.DiagnosticsRequested:
		return "Reporting requested · remote availability unknown"
	}
	return "Stored on this computer"
}
