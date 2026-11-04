package services

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// serveOCR reads text from image files and from the screen through the OCR feature. Each read is rate limited per
// extension; a screen read is limited more tightly, because it holds the screen and a worker for seconds.
func (h *Handler) serveOCR(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityOCR); err != nil {
		return nil, err
	}
	if h.deps.OCR == nil {
		return nil, unavailable("OCR is not available in this build")
	}
	support := ocr.Availability(h.deps.Capabilities)
	switch call.Op {
	case protocol.OpImage:
		return h.readImage(ctx, support, call.Args)
	case protocol.OpScreen:
		return h.readScreenArgs(ctx, support, call.Args)
	case protocol.OpRegion:
		return h.readRegion(ctx, support, call.Args)
	}
	return nil, invalid("ocr has no operation %q", call.Op)
}

func (h *Handler) readScreenArgs(ctx context.Context, support ocr.Features, raw json.RawMessage) (api.OCRResult, error) {
	var args protocol.OCRScreenArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.OCRResult{}, err
	}
	return h.readScreen(ctx, support, args.Options, nil)
}

func (h *Handler) readImage(ctx context.Context, support ocr.Features, raw json.RawMessage) (api.OCRResult, error) {
	if !support.Image {
		return api.OCRResult{}, unavailable("OCR of images is not available on this system")
	}
	var args protocol.OCRImageArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.OCRResult{}, err
	}
	path := args.Input.Source
	if args.Input.File != nil && path == "" {
		path = args.Input.File.Path
	}
	if path == "" {
		return api.OCRResult{}, invalid("OCR needs an image file path")
	}
	if !h.ocrReads.allow() {
		return api.OCRResult{}, rateExceeded("OCR reads")
	}
	result, err := h.deps.OCR.Image(ctx, path, ocrOptionsOf(args.Options))
	return ocrResultOrFailure(result, err)
}

func (h *Handler) readScreen(ctx context.Context, support ocr.Features, options api.OCROptions, region *platform.Rect) (api.OCRResult, error) {
	if !support.Screen {
		return api.OCRResult{}, unavailable("OCR of the screen is not available on this system")
	}
	if !h.ocrReads.allow() || !h.screenReads.allow() {
		return api.OCRResult{}, rateExceeded("screen reads")
	}
	if region != nil {
		result, err := h.deps.OCR.Region(ctx, *region, ocrOptionsOf(options))
		return ocrResultOrFailure(result, err)
	}
	result, err := h.deps.OCR.Screen(ctx, ocrOptionsOf(options))
	return ocrResultOrFailure(result, err)
}

func (h *Handler) readRegion(ctx context.Context, support ocr.Features, raw json.RawMessage) (api.OCRResult, error) {
	var args protocol.OCRRegionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.OCRResult{}, err
	}
	region := platform.Rect{
		X:      int(math.Round(args.Region.X)),
		Y:      int(math.Round(args.Region.Y)),
		Width:  int(math.Round(args.Region.Width)),
		Height: int(math.Round(args.Region.Height)),
	}
	return h.readScreen(ctx, support, args.Options, &region)
}

func ocrOptionsOf(options api.OCROptions) ocr.Options {
	languages := append([]string{}, options.Languages...)
	if options.Language != "" {
		languages = append([]string{options.Language}, languages...)
	}
	return ocr.Options{Languages: languages, Timeout: time.Duration(options.TimeoutMs) * time.Millisecond}
}

// ocrResultOrFailure maps an OCR outcome to the wire form. Refusals keep their code, so an extension can tell a
// missing permission from a bad path.
func ocrResultOrFailure(result platform.TextResult, err error) (api.OCRResult, error) {
	if err != nil {
		return api.OCRResult{}, ocrFailure(err)
	}
	out := api.OCRResult{Text: result.Text, Confidence: result.Confidence, Language: result.Language}
	for _, block := range result.Blocks {
		box := api.Rect{X: float64(block.Box.X), Y: float64(block.Box.Y), Width: float64(block.Box.Width), Height: float64(block.Box.Height)}
		out.Blocks = append(out.Blocks, api.OCRBlock{Text: block.Text, Confidence: block.Confidence, Box: &box})
	}
	return out, nil
}

func ocrFailure(err error) error {
	switch {
	case errors.Is(err, ocr.ErrNotImage), errors.Is(err, ocr.ErrNotAbsolute), errors.Is(err, ocr.ErrEmptyRegion):
		return serviceError{failure: api.Failure{Code: api.CodeInvalid, Message: err.Error()}, cause: api.ErrInvalidInput}
	case errors.Is(err, platform.ErrDenied):
		return serviceError{failure: api.Failure{Code: api.CodePermission, Message: "Nevermind may not record the screen; allow screen recording in system settings"}, cause: api.ErrPermission}
	case errors.Is(err, platform.ErrUnsupported):
		return unavailable("OCR is not available on this system")
	case errors.Is(err, context.DeadlineExceeded):
		return serviceError{failure: api.Failure{Code: api.CodeTimeout, Message: "OCR took too long and was stopped"}, cause: api.ErrNotAvailable}
	}
	return err
}
