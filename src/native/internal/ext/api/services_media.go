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

// DictationStartOptions chooses a microphone and whether system audio is muted while recording.
type DictationStartOptions struct {
	DeviceID                   string `json:"deviceId,omitempty"`
	MuteSystemAudioWhileRecord bool   `json:"muteSystemAudioWhileRecording,omitempty"`
}

// Dictation records speech and returns the hosted transcript. Audio is never exposed to extension
// code. The recording-recovery helpers of the TypeScript API are built-in only and omitted here.
type Dictation interface {
	Status() (DictationStatus, error)
	APIAvailable() (bool, error)
	Devices() ([]DictationDevice, error)
	// Start resolves once the microphone delivers stable audio.
	Start(options DictationStartOptions) error
	// Stop ends capture and returns the transcript.
	Stop() (transcript string, err error)
	Cancel() error
}
