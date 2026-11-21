package search

// AppendedProvider is a Provider whose rows are a second batch for the query: they follow the ranked rows of the
// other providers instead of ranking among them. See mergeAppended.
type AppendedProvider interface {
	Provider
	// Appended reports that the provider's rows follow the ranked rows.
	Appended() bool
}

// appendedRows is a rows provider whose rows are appended.
type appendedRows struct {
	Provider
}

// Appended is true: the rows of an appended provider follow the ranked rows.
func (appendedRows) Appended() bool { return true }

// NewAppendedRowsProvider returns a provider whose rows follow the ranked rows of the other providers, as NewRowsProvider
// returns one whose rows are ranked. A search that asks a model for more matches uses it, so the local matches stay on top.
func NewAppendedRowsProvider(id string, rows RowsFunc) Provider {
	return appendedRows{Provider: NewRowsProvider(id, rows)}
}

// isAppended reports whether p is an appended provider.
func isAppended(p Provider) bool {
	appended, ok := p.(AppendedProvider)
	return ok && appended.Appended()
}
