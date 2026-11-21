// Package fixtures builds deterministic data for tests and dev-only fixture views: applications with icons,
// a five-thousand-file tree, clipboard history with images and concealed entries, chats with streamed
// transcripts, a fake Nevermind backend with switchable failure modes, and extension source files.
//
// Every builder is a pure function of its arguments: no time.Now, no unseeded randomness. Calling a builder
// twice yields identical output, which the tests check. Layer: unranked. Production packages must not import
// this package; the platform grep test enforces that.
package fixtures
