// Package dictation records the microphone, encodes it as Opus, cuts it at natural pauses, sends each piece to the
// backend's transcription route while the user goes on speaking, and returns the joined text for the caller to paste.
// It owns the state of one dictation at a time (idle, recording, transcribing) and reports the level for the indicator. The device, the Opus library and the
// backend are ports, so tests run with fakes. Layer: service tier (with jobs, shortcuts, windowmgmt, update and diag).
package dictation
