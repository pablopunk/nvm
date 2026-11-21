package extai

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// ClipboardAttachments reads the clipboard as attachments: its text, or else its image. An image is recognised as
// text when as is ocr. A clipboard with neither gives no attachment.
func (s *Service) ClipboardAttachments(ctx context.Context, as, title string) ([]api.Attachment, error) {
	text, err := s.deps.Clipboard.ReadText()
	if err != nil {
		return nil, err
	}
	if text != "" {
		return []api.Attachment{{Kind: "text", Title: sectionTitle(title, "Clipboard"), Text: limitText(text)}}, nil
	}
	png, err := s.deps.Clipboard.ReadImagePNG()
	if errors.Is(err, platform.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if as == "ocr" {
		return s.clipboardOCR(ctx, png, title)
	}
	return []api.Attachment{{Kind: "image", Title: sectionTitle(title, "Clipboard Image"), DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}}, nil
}

func (s *Service) clipboardOCR(ctx context.Context, png []byte, title string) ([]api.Attachment, error) {
	if s.deps.OCR == nil {
		return nil, InputError("OCR is not available on this system")
	}
	result, err := s.deps.OCR.ImagePNG(ctx, png, ocr.Options{})
	if err != nil {
		return nil, err
	}
	return []api.Attachment{{Kind: "text", Title: sectionTitle(title, "OCR Text"), Text: limitText(result.Text)}}, nil
}

// OCRAttachment recognises the text of one image file as a text attachment.
func (s *Service) OCRAttachment(ctx context.Context, path, title string) (api.Attachment, error) {
	text, err := s.ocrSource(ctx, path)
	if err != nil {
		return api.Attachment{}, err
	}
	return api.Attachment{Kind: "text", Title: sectionTitle(title, "OCR Text"), Text: limitText(text)}, nil
}
