// Package dictation records the microphone, encodes it as Opus, sends each minute of speech to the backend's
// transcription route and returns the text for the caller to paste. It owns the state of one dictation at a time
// (idle, recording, transcribing) and reports the level for the indicator. The device, the Opus library and the
// backend are ports, so tests run with fakes. Layer: service tier (with jobs, shortcuts, windowmgmt, update and diag).
package dictation
