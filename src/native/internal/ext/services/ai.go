package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveAI answers the model calls of ctx.AI. Every operation needs the ai capability. A call whose attachments read
// files, the clipboard or OCR also needs the capability of that read.
func (h *Handler) serveAI(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityAI); err != nil {
		return nil, err
	}
	if h.deps.AI == nil {
		return nil, unavailable("AI is not available in this build")
	}
	switch call.Op {
	case protocol.OpIsAvailable:
		return protocol.AIAvailability{Available: h.deps.AI.Available()}, nil
	case protocol.OpPrepare:
		return h.prepareAI(ctx, call.Args)
	case protocol.OpAsk:
		return h.askAI(ctx, call.Args)
	case protocol.OpDecide:
		return h.decideAI(ctx, call.Args)
	case protocol.OpStreamOpen:
		return h.openAIStream(call.Args)
	case protocol.OpStreamNext:
		return h.nextAIStream(ctx, call.Args)
	case protocol.OpStreamAbort:
		return h.abortAIStream(call.Args)
	case protocol.OpSessionReset:
		return h.resetAISession(call.Args)
	case protocol.OpAttachClipboard:
		return h.clipboardAttachments(ctx, call.Args)
	case protocol.OpAttachOCR:
		return h.ocrAttachment(ctx, call.Args)
	}
	return nil, invalid("ai has no operation %q", call.Op)
}

func (h *Handler) prepareAI(ctx context.Context, raw json.RawMessage) (struct{}, error) {
	var args protocol.AIPrepareArgs
	if err := decodeArgs(raw, &args); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, aiFailure(h.deps.AI.Warm(ctx, args.Options))
}

func (h *Handler) askAI(ctx context.Context, raw json.RawMessage) (string, error) {
	var args protocol.AIAskArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if err := h.requireAIReads(args.Options.Attachments); err != nil {
		return "", err
	}
	if err := h.allowAICall(); err != nil {
		return "", err
	}
	if args.Session != "" {
		return h.sessionAsk(ctx, args)
	}
	completed, err := h.deps.AI.Complete(ctx, extai.Call{Options: args.Options, Prompt: args.Prompt})
	if err != nil {
		return "", aiFailure(err)
	}
	return completed.Text, nil
}

func (h *Handler) decideAI(ctx context.Context, raw json.RawMessage) (api.DecisionResult, error) {
	var args protocol.AIDecideArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.DecisionResult{}, err
	}
	if err := h.requireAIReads(args.Request.Attachments); err != nil {
		return api.DecisionResult{}, err
	}
	if err := h.allowAICall(); err != nil {
		return api.DecisionResult{}, err
	}
	result, err := h.deps.AI.Decide(ctx, args.Request)
	if err != nil {
		return api.DecisionResult{}, aiFailure(err)
	}
	return result, nil
}

// requireAIReads refuses attachments that read files, the clipboard or OCR without the capability of that read.
func (h *Handler) requireAIReads(attachments []api.Attachment) error {
	if extai.NeedsFiles(attachments) {
		if err := h.require(api.CapabilityDesktopFiles); err != nil {
			return err
		}
	}
	if extai.NeedsOCR(attachments) {
		if err := h.require(api.CapabilityOCR); err != nil {
			return err
		}
	}
	return nil
}

// allowAICall takes one call from the extension's AI rate limit.
func (h *Handler) allowAICall() error {
	if !h.aiCalls.allow() {
		return aiRateExceeded()
	}
	return nil
}
