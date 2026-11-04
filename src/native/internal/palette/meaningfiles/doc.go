// Package meaningfiles is the Find Files by Meaning view (WP4.2). The name matches of the Files index show at once.
// A row offers to send the names of the recent files to the decision model for the query the user typed; the matches
// join the list only after that choice, and only for that query. A new query starts again from the name matches.
//
// Layer: palette. The view takes its providers as ports, so it does not know the decision client: the app wires the
// name matches and the decision provider, and tests answer them with fixed rows.
package meaningfiles
