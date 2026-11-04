// Package commands runs the AI commands on the selected text and on a clipboard image: Fix Selected Text with AI
// (a proofread that streams through the chat service and is replaced into the source app), Triage Selected Text
// with AI (typed decision answers shown in the indicator) and Classify Clipboard Image with AI. It reads the selection
// through the platform's copy gesture and restores the clipboard it disturbed. Every method runs on the UI thread and
// hands its background work to Options.Background; results come back through Options.Dispatch.
package commands
