// Package filesearch finds files by meaning (WP4.2): the decision model judges the names of the most recent files
// against the user's query, and the matches join the Files search as rows.
//
// The method is Electron's ("Find Files by Meaning"): one predicate question per candidate name, answered by the
// backend decision route. There is no embedding index, so nothing is indexed in the background and nothing is
// stored. The candidates are the CandidateLimit most recent file names; paths, contents and thumbnails never leave
// the machine. The decision is made only when the user asks for it, never on each keystroke.
//
// Layer: ai. The package imports the backend client's decision types, the file index and the search package. The
// decision client is reached through the Decider interface, so tests answer it without the network.
package filesearch
