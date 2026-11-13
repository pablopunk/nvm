package extai

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// MaxAttachments bounds the attachments of one call. A decision refuses more; a prompt keeps the first ones.
const MaxAttachments = 8

// Limits of text and image attachments, the values 1.x uses.
const (
	textAttachmentChars = 80_000
	askImageBytes       = 8 << 20
	decisionImageBytes  = 2 << 20
	decisionImageCount  = 4
)

// attachmentContent is what the attachments of a call add to it: text sections in order, and images.
type attachmentContent struct {
	sections []string
	images   []backend.DecisionImage
}

func (c attachmentContent) text() string {
	return strings.Join(c.sections, "\n\n")
}

// readAttachments reads attachments into text sections and images. A decision reads them strictly: an attachment it
// cannot read fails the call instead of being left out.
func (s *Service) readAttachments(ctx context.Context, attachments []api.Attachment, decision bool) (attachmentContent, error) {
	if decision && len(attachments) > MaxAttachments {
		return attachmentContent{}, inputErrorf("AI decisions accept at most %d attachments", MaxAttachments)
	}
	var content attachmentContent
	for _, attachment := range attachments {
		if err := s.readAttachment(ctx, &content, attachment, decision); err != nil {
			return attachmentContent{}, err
		}
	}
	if decision && len(content.images) > decisionImageCount {
		return attachmentContent{}, inputErrorf("Decisions accept at most %d images; %d were attached.", decisionImageCount, len(content.images))
	}
	return content, nil
}

func (s *Service) readAttachment(ctx context.Context, content *attachmentContent, attachment api.Attachment, decision bool) error {
	switch attachment.Kind {
	case "text":
		content.sections = append(content.sections, sectionOf(attachment.Title, limitText(attachment.Text)))
		return nil
	case "image":
		return s.readImageAttachment(ctx, content, attachment, decision)
	case "file":
		return s.readFileAttachment(ctx, content, attachment, decision)
	}
	if decision {
		return InputError("Unsupported decision attachment.")
	}
	return nil
}

func (s *Service) readImageAttachment(ctx context.Context, content *attachmentContent, attachment api.Attachment, decision bool) error {
	source := attachment.DataURL
	if source == "" {
		source = attachment.Path
	}
	if source == "" {
		return InputError("An image attachment needs a path or a data URL.")
	}
	image, err := s.imageFrom(source, decision, len(content.images))
	if err != nil {
		return err
	}
	content.images = append(content.images, image)
	if attachment.OCR {
		return s.addOCR(ctx, content, source, sectionTitle(attachment.Title, "OCR"))
	}
	return nil
}

func (s *Service) readFileAttachment(ctx context.Context, content *attachmentContent, attachment api.Attachment, decision bool) error {
	source := attachment.Path
	if source == "" && attachment.File != nil {
		source = attachment.File.Path
	}
	if source == "" {
		if decision {
			return InputError("Decision file attachment has no path.")
		}
		return nil
	}
	path, err := s.absolutePath(source)
	if err != nil {
		return err
	}
	asImage := (attachment.As == "image" || (attachment.As == "" && isImagePath(path))) && attachment.As != "text"
	if decision && !asImage && !attachment.OCR && attachment.As != "ocr" && (attachment.As == "metadata" || !isTextLikePath(path)) {
		return inputErrorf("Attachment %s cannot be read as text or an image, so it was not sent.", baseName(path))
	}
	if asImage {
		image, err := s.imageFile(path, decision, len(content.images))
		if err != nil {
			return err
		}
		content.images = append(content.images, image)
	}
	if !asImage || attachment.As == "text" || attachment.As == "metadata" {
		section, err := s.fileSection(path, attachment, decision)
		if err != nil {
			return err
		}
		content.sections = append(content.sections, section)
	}
	if attachment.OCR || attachment.As == "ocr" {
		return s.addOCR(ctx, content, path, sectionTitle(attachment.Title, "OCR "+baseName(path)))
	}
	return nil
}

// fileSection is the header of a file attachment, with its text when the file is text and the attachment asks for it.
func (s *Service) fileSection(path string, attachment api.Attachment, decision bool) (string, error) {
	title := sectionTitle(attachment.Title, baseName(path))
	info, err := s.deps.Files.Stat(path)
	if err != nil {
		if decision {
			return "", inputErrorf("Attachment %s cannot be read as text or an image, so it was not sent.", baseName(path))
		}
		return fmt.Sprintf("### %s\nMissing file: %s", title, path), nil
	}
	mime, known := mimeOf(path)
	if !known {
		mime = "unknown"
	}
	header := fmt.Sprintf("### %s\nPath: %s\nMIME: %s\nSize: %d bytes", title, path, mime, info.Size())
	if attachment.As == "metadata" || !isTextLikePath(path) {
		return header, nil
	}
	data, _, err := s.deps.Files.ReadFilePrefix(path, textAttachmentChars*4)
	if err != nil {
		return "", err
	}
	return header + "\n\n" + limitText(string(data)), nil
}

// addOCR adds the text recognised in source as a section.
func (s *Service) addOCR(ctx context.Context, content *attachmentContent, source, title string) error {
	text, err := s.ocrSource(ctx, source)
	if err != nil {
		return err
	}
	content.sections = append(content.sections, sectionOf(title, limitText(text)))
	return nil
}

// sectionTitle is title, or fallback when the attachment has none.
func sectionTitle(title, fallback string) string {
	if title != "" {
		return title
	}
	return fallback
}

// sectionOf is a text section with its heading.
func sectionOf(title, body string) string {
	if title == "" {
		return body
	}
	return "### " + title + "\n" + body
}

// limitText cuts text to the attachment size limit, marking the cut.
func limitText(text string) string {
	runes := []rune(text)
	if len(runes) <= textAttachmentChars {
		return text
	}
	return string(runes[:textAttachmentChars]) + "\n[truncated]"
}
