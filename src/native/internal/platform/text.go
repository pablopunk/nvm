package platform

import "context"

// TextBlock is one recognized line of text and where it sits in the image. Box is in the image's own pixels, with
// the origin at the top left; Confidence is between 0 and 1.
type TextBlock struct {
	Text       string
	Confidence float64
	Box        Rect
}

// TextResult is the text recognized in one image. Confidence is the mean of the blocks' confidences and is meaningful
// only when Blocks is not empty. Language is the detected language when the engine can identify it, otherwise "".
type TextResult struct {
	Text       string
	Confidence float64
	Language   string
	Blocks     []TextBlock
}

// TextRecognizer is the intent "read the text in an image file, on this machine". Recognition never leaves the
// machine. Where the OS has no engine the capability text.recognize is absent and Recognize returns ErrUnsupported.
// The context's deadline bounds the run; a recognizer that outlives it is killed and returns the context error.
type TextRecognizer interface {
	// Recognize reads the text of the image at path. languages are BCP 47 tags or engine language codes; an empty list
	// lets the engine choose.
	Recognize(ctx context.Context, path string, languages []string) (TextResult, error)
}

// ScreenCapturer is the intent "save what the screen shows, or one rectangle of it, as a PNG file". It needs the
// capability screen.capture; where the OS cannot capture, Capture returns ErrUnsupported. A nil region captures the
// whole main display. Capture returns ErrDenied when the OS refused screen recording.
type ScreenCapturer interface {
	// Capture writes a PNG to dst, which the caller chose and removes.
	Capture(ctx context.Context, dst string, region *Rect) error
}
