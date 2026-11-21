package geometry

// Outcome classifies a frame write by what the window actually reached. Only Applied is a clean
// success; Partial and Failed must be reported with their message.
type Outcome int

const (
	// Applied means the observed frame matches the requested frame within tolerance.
	Applied Outcome = iota
	// Partial means the window moved or resized but did not reach the requested frame.
	Partial
	// Failed means the window still has its original frame, so the app refused the change.
	Failed
)

// String returns the lowercase outcome name used in logs and tests.
func (o Outcome) String() string {
	switch o {
	case Applied:
		return "applied"
	case Partial:
		return "partial"
	}
	return "failed"
}

// Verdict is an Outcome with the palette message to show for it. Applied has no message.
type Verdict struct {
	Outcome Outcome
	Message string
}

// Verify decides what a write achieved. requested is the frame asked for, observed the frame read
// back afterwards, original the frame before the write, and minSize the smallest size the app is
// known to enforce (zero when unknown).
//
// A request smaller than the app's minimum can never be reached exactly, so when the observed frame
// equals the minimum-clamped request the result is Partial with an explanation, not Applied. A
// frame equal to the original is Failed. Everything else that differs is Partial.
func Verify(requested, observed, original Rect, minSize Size) Verdict {
	if FramesMatch(observed, requested, ExactTolerancePoints) {
		return Verdict{Outcome: Applied}
	}
	if floor, enforced := minimumClampedRequest(requested, minSize); enforced && FramesMatch(observed, floor, ExactTolerancePoints) {
		return Verdict{Outcome: Partial, Message: MessagePartialMinimumSize}
	}
	if FramesMatch(observed, original, ExactTolerancePoints) {
		return Verdict{Outcome: Failed, Message: MessageUnapplied}
	}
	return Verdict{Outcome: Partial, Message: MessagePartialGeneric}
}

// minimumClampedRequest raises the requested size to the enforced minimum. enforced reports whether
// any dimension was raised.
func minimumClampedRequest(requested Rect, minSize Size) (Rect, bool) {
	clamped := requested
	enforced := false
	if minSize.Width > requested.Width {
		clamped.Width = minSize.Width
		enforced = true
	}
	if minSize.Height > requested.Height {
		clamped.Height = minSize.Height
		enforced = true
	}
	return clamped, enforced
}

// NeedsOverflowCorrection reports whether a partial write left part of the window outside its
// display, in which case the engine issues one position-only correction (writeWithCorrection).
func NeedsOverflowCorrection(outcome Outcome, observed, workArea Rect) bool {
	return outcome == Partial && !IsFrameInside(observed, workArea)
}

// OverflowCorrectionFrame is the position-only frame that pulls an overflowing window back inside
// its display: the requested origin, the size the app actually accepted, clamped to the area.
func OverflowCorrectionFrame(requested, observed, workArea Rect) Rect {
	return ClampFrameInside(Rect{
		X:      requested.X,
		Y:      requested.Y,
		Width:  observed.Width,
		Height: observed.Height,
	}, workArea)
}
