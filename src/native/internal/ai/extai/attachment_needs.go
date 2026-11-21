package extai

import "github.com/pablopunk/nvm/native/internal/ext/api"

// NeedsFiles reports whether reading attachments touches files on disk, so the caller can check the files capability.
func NeedsFiles(attachments []api.Attachment) bool {
	for _, attachment := range attachments {
		if attachment.Kind == "file" || (attachment.Kind == "image" && attachment.DataURL == "" && attachment.Path != "") {
			return true
		}
	}
	return false
}

// NeedsOCR reports whether reading attachments recognises text in images, so the caller can check the OCR capability.
func NeedsOCR(attachments []api.Attachment) bool {
	for _, attachment := range attachments {
		if attachment.OCR || attachment.As == "ocr" {
			return true
		}
	}
	return false
}
