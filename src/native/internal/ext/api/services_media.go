package api

// OCRInput names an image: a path, a file URL, a PNG data URL (for example from ReadImage), or a file reference.
type OCRInput struct {
	Source string   `json:"source,omitempty"`
	File   *FileRef `json:"file,omitempty"`
}

// OCROptions selects recognition languages. Empty means the host default.
type OCROptions struct {
	Language  string   `json:"language,omitempty"`
	Languages []string `json:"languages,omitempty"`
	TimeoutMs int64    `json:"timeoutMs,omitempty"`
}

// OCRBlock is one recognised text block with its box in image pixels.
type OCRBlock struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence,omitempty"`
	Box        *Rect   `json:"box,omitempty"`
}

// OCRResult is recognised text. Confidence and Language are zero when the engine does not report them.
type OCRResult struct {
	Text       string     `json:"text"`
	Confidence float64    `json:"confidence,omitempty"`
	Language   string     `json:"language,omitempty"`
	Blocks     []OCRBlock `json:"blocks,omitempty"`
}

// OCR recognises text. Declare the ocr capability for review; check System().HasCapability("ocr").
// Screen and Region need screen-recording permission from the OS.
type OCR interface {
	Image(input OCRInput, options OCROptions) (OCRResult, error)
	// Screen captures the visible screen and recognises its text.
	Screen(options OCROptions) (OCRResult, error)
	// Region captures a rectangle in physical screen coordinates.
	Region(region Rect, options OCROptions) (OCRResult, error)
}

// DictationStatus is the current state of the built-in speech capture.
type DictationStatus string

const (
	DictationIdle         DictationStatus = "idle"
	DictationRecording    DictationStatus = "recording"
	DictationTranscribing DictationStatus = "transcribing"
	DictationError        DictationStatus = "error"
)

// DictationDevice is a microphone the user can pick.
type DictationDevice struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	IsDefault bool   `json:"isDefault"`
}

// DictationStartOptions chooses a microphone and whether system audio is muted while recording. An empty DeviceID
// is the system default microphone.
type DictationStartOptions struct {
	DeviceID                   string `json:"deviceId,omitempty"`
	MuteSystemAudioWhileRecord bool   `json:"muteSystemAudioWhileRecording,omitempty"`
}

// SavedDictation is a saved dictation whose transcript did not complete. Its audio stays on this device and is
// only sent again by Retry.
type SavedDictation struct {
	ID           string `json:"id"`
	CreatedAtMs  int64  `json:"createdAtMs"`
	SegmentCount int    `json:"segmentCount"`
}

// Dictation records speech and returns the hosted transcript. Audio is never exposed to extension code. Start resolves
// once the microphone delivers stable audio, and Stop is the call that waits for the hosted transcription, so an
// action that runs it should declare Background. Recordings, Retry and DeleteRecording recover a dictation whose
// transcription failed; a saved recording is deleted once its text is saved.
type Dictation interface {
	Status() (DictationStatus, error)
	// APIAvailable reports whether authenticated hosted transcription is available for this client.
	APIAvailable() (bool, error)
	Devices() ([]DictationDevice, error)
	Start(options DictationStartOptions) error
	// Stop ends capture and returns the hosted transcript.
	Stop() (transcript string, err error)
	Cancel() error
	Recordings() ([]SavedDictation, error)
	// Retry transcribes a saved recording again. Each retry is a new, billable request.
	Retry(id string) (transcript string, err error)
	DeleteRecording(id string) error
}
