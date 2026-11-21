package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// DictationEngine is the host's one-dictation-at-a-time recorder. Start opens the microphone and Stop returns the
// hosted transcript; the audio stays in the host. Recordings are the dictations whose transcript did not complete.
// Errors carry the user's words as their message, and the api cause that classifies them.
type DictationEngine interface {
	Status() api.DictationStatus
	APIAvailable() bool
	Devices() ([]api.DictationDevice, error)
	Start(options api.DictationStartOptions) error
	Stop() (string, error)
	Cancel() error
	Recordings() ([]api.SavedDictation, error)
	Retry(ctx context.Context, id string) (string, error)
	DeleteRecording(id string) error
}

// serveDictation answers the dictation service. Every operation needs the dictation capability.
func (h *Handler) serveDictation(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDictation); err != nil {
		return nil, err
	}
	if h.deps.Dictation == nil {
		return nil, unavailable("dictation is not available in this build")
	}
	engine := h.deps.Dictation
	switch call.Op {
	case protocol.OpStatus:
		return engine.Status(), nil
	case protocol.OpAPIAvailable:
		return engine.APIAvailable(), nil
	case protocol.OpDevices:
		return engine.Devices()
	case protocol.OpStart:
		var options api.DictationStartOptions
		if err := decodeArgs(call.Args, &options); err != nil {
			return nil, err
		}
		return nil, h.startDictation(options)
	case protocol.OpStop:
		return engine.Stop()
	case protocol.OpCancel:
		return nil, engine.Cancel()
	case protocol.OpRecordings:
		return engine.Recordings()
	case protocol.OpRetry:
		var args protocol.DictationRecordingArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return engine.Retry(ctx, args.ID)
	case protocol.OpDeleteRecording:
		var args protocol.DictationRecordingArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return nil, engine.DeleteRecording(args.ID)
	}
	return nil, invalid("dictation has no operation %q", call.Op)
}
