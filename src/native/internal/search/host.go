package search

// Host is what an action runs against: the palette that shows the row. Actions reach the palette only
// through these methods, so a feature never names the palette's type (search sits below it in the layer
// order, and features sit below search).
type Host interface {
	// Hide closes the palette window.
	Hide()
	// Pop returns to the view below the current one; it does nothing at the root.
	Pop()
	// Root returns to the first view of the palette, leaving every view above it.
	Root()
	// Push opens a view above the current one. The argument is the palette's view type; the palette
	// panics on anything else, because only the composition of views can build one.
	Push(view any)
	// Toast shows a one-line message under the search field.
	Toast(msg string)
	// Refresh runs the current search again and keeps the cursor where it is.
	Refresh()
	// Query returns the text of the search field.
	Query() string
	// SetQuery replaces the text of the search field and searches again.
	SetQuery(q string)
}

// ActionLoadingHost acknowledges asynchronous actions without re-running providers. The returned completion clears
// feedback and reports whether the action still belongs to the visible navigation/query; obsolete results are ignored.
type ActionLoadingHost interface {
	BeginAction() func() bool
}
