// Package charsearch is the meaning-based second batch of the Emoji & Symbols picker (1.x's semantic character search).
// A typed text that rests for a moment is sent to the decision model, which narrows the catalog in three steps: which
// categories could match, which characters each category offers, and whether each shortlisted character matches.
//
// Layer: ai. The package takes the catalog as Records and the decision client as an interface, so the app wires the
// signed-in account in and the tests answer with a fake model.
package charsearch
