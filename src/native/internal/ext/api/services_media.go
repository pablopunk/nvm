package api

// OCRInput names an image: a path (a leading "~" is the home folder), a file: URL, an image data URL (for example from
// ReadImage), or a file reference.
type OCRInput struct {
	Source string   `json:"source,omitempty"`
	File   *FileRef `json:"file,omitempty"`
}

// OCROptions supplies language hints. The bundled recognizer reads Latin scripts together and does not require hints.
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

// OCR recognises text. Declare ocr; the host enforces that declaration.
// Probe System().HasCapability("ocr") for images or "ocr.screen" for captures.
// Probes describe wired implementations, independently of permission and temporary-folder readiness.
// Screen and Region need screen-recording permission from the OS.
type OCR interface {
	Image(input OCRInput, options OCROptions) (OCRResult, error)
	// Screen captures the visible screen and recognises its text.
	Screen(options OCROptions) (OCRResult, error)
	// Region captures a rectangle in physical screen coordinates.
	Region(region Rect, options OCROptions) (OCRResult, error)
}

// CameraKind says what a camera is, so a picker can group, order or label it.
type CameraKind string

const (
	CameraBuiltIn    CameraKind = "built-in"
	CameraExternal   CameraKind = "external"
	CameraContinuity CameraKind = "continuity" // a Continuity Camera or a Desk View camera
	CameraVirtual    CameraKind = "virtual"    // a camera software draws, such as OBS
	CameraOther      CameraKind = "other"
)

// CameraDevice is a camera the user can pick. ID is stable for the same camera across launches: store it, and pass it
// as CameraSpec.DeviceID or as the Target of CameraSelectDevice to show that camera.
type CameraDevice struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	IsDefault bool       `json:"isDefault"`
	Kind      CameraKind `json:"kind"`
}

// Cameras lists the cameras of this computer, the system default first. Declare camera. Probe
// System().HasCapability("camera.devices"); where the OS cannot list them, Devices fails as unsupported and camera
// views still offer their own switcher. Listing never asks for camera permission and never turns a camera on, so it is
// safe in RootItems, SearchItems and LoadItems. A camera submenu is a list view of these devices whose rows open a
// camera view with CameraSpec.DeviceID set, or, inside an open camera view, run CameraSelectDevice. Camera views
// already add a Cameras section to their own action menu, so an extension does not need to repeat it there.
type Cameras interface {
	Devices() ([]CameraDevice, error)
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
	// IndicatorID binds host-owned microphone levels to this extension’s recording indicator.
	// Show that indicator with IndicatorRecording first. Levels never cross into extension code;
	// a replacement indicator or a new recording starts a fresh trace. Empty disables the binding.
	IndicatorID                string `json:"indicatorId,omitempty"`
	DeviceID                   string `json:"deviceId,omitempty"`
	MuteSystemAudioWhileRecord bool   `json:"muteSystemAudioWhileRecording,omitempty"`
}

// SavedDictation is a saved dictation whose transcript did not complete. Its audio stays on this device and is
// only sent again by Retry.
type SavedDictation struct {
	ID           string `json:"id"`
	CreatedAtMs  int64  `json:"createdAtMs"`
	SegmentCount int    `json:"segmentCount"`
	Failure      string `json:"failure,omitempty"`
}

// Dictation records speech and returns the hosted transcript. Audio is never exposed to extension code. Start resolves
// after a microphone frame is captured (and Bluetooth input is ready), and Stop is the call that waits for the hosted transcription, so an
// action that runs it should declare Background. The host retries passing failures before it reports one, and keeps
// the audio of every dictation that holds sound until its transcript is in: Recordings, Retry and DeleteRecording
// recover a dictation that failed, was cancelled or was cut off; a saved recording is deleted once its text is saved.
type Dictation interface {
	Status() (DictationStatus, error)
	// APIAvailable reports whether authenticated hosted transcription is available for this client.
	APIAvailable() (bool, error)
	Devices() ([]DictationDevice, error)
	Start(options DictationStartOptions) error
	// Stop ends capture and returns the hosted transcript.
	Stop() (transcript string, err error)
	// Cancel ends capture without transcribing. Audio already captured stays in Recordings, marked as cancelled.
	Cancel() error
	Recordings() ([]SavedDictation, error)
	// Retry transcribes a saved recording again. Each retry is a new, billable request.
	Retry(id string) (transcript string, err error)
	DeleteRecording(id string) error
}
