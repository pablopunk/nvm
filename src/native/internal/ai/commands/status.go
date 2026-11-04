package commands

// Tone is how an indicator status reads: work in progress, a result, or a failure.
type Tone string

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
}

// Result is a finished command the palette shows. Text returns the text to show, read on every frame. Replace, when it
// is set, replaces the source selection with the text and hides the palette; a result without it can only be copied,
// through Copy.
type Result struct {
	Title   string
	Text    func() string
	Replace func(text string)
	Copy    func(text string)
}
