package commands

// Tone is how an indicator status reads: work in progress, a result, or a failure.
type Tone string

// Activity describes work whose visual treatment is independent of its localized copy.
type Activity string

const (
	ActivityNone         Activity = ""
	ActivityRecording    Activity = "recording"
	ActivityTranscribing Activity = "transcribing"
	ActivityLoading      Activity = "loading"
)

// The tones an indicator status can take.
const (
	ToneLoading Tone = "loading"
	ToneSuccess Tone = "success"
	ToneError   Tone = "error"
)

// Status is what the indicator shows: a title, one line of copy and its tone.
type Status struct {
	Title    string
	Subtitle string
	Tone     Tone
	Activity Activity
}
