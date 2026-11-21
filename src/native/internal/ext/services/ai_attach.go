package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// clipboardAttachments reads the clipboard for ctx.AI attachments. It needs the desktop clipboard capability.
func (h *Handler) clipboardAttachments(ctx context.Context, raw json.RawMessage) (protocol.AIAttachments, error) {
	var args protocol.AIClipboardArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIAttachments{}, err
	}
	if err := h.require(api.CapabilityDesktopClipboard); err != nil {
		return protocol.AIAttachments{}, err
	}
	attachments, err := h.deps.AI.ClipboardAttachments(ctx, args.As, "")
	if err != nil {
		return protocol.AIAttachments{}, aiFailure(err)
	}
	return protocol.AIAttachments{Attachments: attachments}, nil
}

// ocrAttachment recognises the text of one image file for ctx.AI. It needs the ocr capability and the OCR rate.
func (h *Handler) ocrAttachment(ctx context.Context, raw json.RawMessage) (protocol.AIAttachments, error) {
	var args protocol.AIOCRImageArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIAttachments{}, err
	}
	if err := h.require(api.CapabilityOCR); err != nil {
		return protocol.AIAttachments{}, err
	}
	if args.Path == "" {
		return protocol.AIAttachments{}, invalid("OCR needs an image file path")
	}
	if !h.ocrReads.allow() {
		return protocol.AIAttachments{}, rateExceeded("OCR reads")
	}
	attachment, err := h.deps.AI.OCRAttachment(ctx, args.Path, args.Title)
	if err != nil {
		return protocol.AIAttachments{}, aiFailure(err)
	}
	return protocol.AIAttachments{Attachments: []api.Attachment{attachment}}, nil
}
