package services

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// snapshotPermissionDenied preserves the public no-thumbnail result while notifying the host cache.
type snapshotPermissionDenied struct{}

func (snapshotPermissionDenied) Error() string { return "snapshot source access denied" }

func (h *Handler) serveMarked(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if h.closed.Load() {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: errHandlerClosed.Error()}}
	}
	result, err := h.dispatch(ctx, call)
	if err != nil {
		if _, ok := err.(snapshotPermissionDenied); ok {
			data, _ := json.Marshal(result)
			return protocol.ServiceReply{Data: data, SnapshotPermissionFailure: &api.Failure{Code: api.CodePermission, Message: "File access was denied."}}
		}
		failure := failureOf(err)
		return protocol.ServiceReply{Failure: &failure}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeInternal, Message: "the service result cannot be encoded"}}
	}
	return protocol.ServiceReply{Data: data}
}
