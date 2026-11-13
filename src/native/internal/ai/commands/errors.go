package commands

// userError is a failure whose text is already the copy the user sees.
type userError string

func (e userError) Error() string { return string(e) }

// The failures the correction names. Each carries the words the indicator shows.
const (
	ErrNoCorrection      = userError("AI returned no corrected text")
	ErrCorrectionRefused = userError("AI did not return a valid correction")
)
