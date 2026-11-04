package backend

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

var sampleWebM = append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{7}, 64)...)

func TestTranscribeSendsTheWebMFileWithItsKeyAndReturnsTheText(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, `{"text":" hello there "}`))
	text, err := h.client.Transcribe(context.Background(), TranscriptionRequest{OperationID: "dictation-abc", CancellationID: "dictation-root", Audio: sampleWebM})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != " hello there " {
		t.Fatalf("text = %q, want the backend's text unchanged for the caller to trim", text)
	}
	request := h.http.Requests()[0]
	if request.Method != "POST" || request.URL != "https://api.nvm.fyi/api/v1/audio/transcriptions" {
		t.Fatalf("request = %s %s, want POST to the transcriptions route", request.Method, request.URL)
	}
	if got := request.Header.Get("Content-Type"); got != "audio/webm" {
		t.Fatalf("Content-Type = %q, want audio/webm", got)
	}
	if got := request.Header.Get("Idempotency-Key"); got != "dictation-abc" {
		t.Fatalf("Idempotency-Key = %q, want the operation id", got)
	}
	if got := request.Header.Get(operationIDHeader); got != "dictation-root" {
		t.Fatalf("%s = %q, want the dictation's cancellation id", operationIDHeader, got)
	}
	if got := request.Header.Get(preparationHeader); got != "" {
		t.Fatalf("%s = %q on an unprepared request, want none", preparationHeader, got)
	}
	if !bytes.Equal(request.Body, sampleWebM) {
		t.Fatalf("body is not the raw WebM file")
	}
	if got := request.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Fatalf("Authorization = %q, want the bearer token", got)
	}
}

func TestTranscribeSendsThePreparationHeaderOnlyWhenPrepared(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, `{"text":"a"}`))
	if _, err := h.client.Transcribe(context.Background(), TranscriptionRequest{OperationID: "dictation-abc", CancellationID: "dictation-abc", Prepared: true, Audio: sampleWebM}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if got := h.http.Requests()[0].Header.Get(preparationHeader); got != "dictation-abc" {
		t.Fatalf("%s = %q, want the prepared operation id", preparationHeader, got)
	}
}

func TestTranscribeReportsCreditAndFreePoolRefusals(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		kind   Kind
		code   string
	}{
		{"credits", http.StatusPaymentRequired, `{"error":{"type":"insufficient_credits","message":"Not enough credits for API dictation."}}`, KindInsufficientCredits, "insufficient_credits"},
		{"free pool paused", http.StatusServiceUnavailable, `{"error":{"type":"free_spend_cap_reached","message":"paused"}}`, KindServer, "free_spend_cap_reached"},
		{"per-request limit", http.StatusBadRequest, `{"error":{"type":"audio_cost_limit","message":"This recording exceeds the 30-credit per-request limit."}}`, KindRejected, "audio_cost_limit"},
		{"expired token", http.StatusUnauthorized, `{"error":{"type":"unauthorized","message":"Sign in to use API dictation."}}`, KindUnauthorized, "unauthorized"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.http.Script(jsonBody(tt.status, tt.body))
			_, err := h.client.Transcribe(context.Background(), TranscriptionRequest{OperationID: "k", Audio: sampleWebM})
			if !IsKind(err, tt.kind) {
				t.Fatalf("error = %v, want kind %s", err, tt.kind)
			}
			if typed := asError(err); typed == nil || typed.Code != tt.code {
				t.Fatalf("error code = %+v, want %s", typed, tt.code)
			}
		})
	}
}

func TestTranscribeRefusesToSendAnUnusableRequest(t *testing.T) {
	tests := []struct {
		name string
		req  TranscriptionRequest
	}{
		{"empty audio", TranscriptionRequest{OperationID: "k"}},
		{"oversized audio", TranscriptionRequest{OperationID: "k", Audio: make([]byte, maxTranscriptionBytes+1)}},
		{"missing key", TranscriptionRequest{Audio: sampleWebM}},
		{"oversized key", TranscriptionRequest{OperationID: strings.Repeat("k", 201), Audio: sampleWebM}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.client.Transcribe(context.Background(), tt.req); !IsKind(err, KindRejected) {
				t.Fatalf("error = %v, want KindRejected", err)
			}
			if n := len(h.http.Requests()); n != 0 {
				t.Fatalf("made %d requests for an unusable recording, want none", n)
			}
		})
	}
}

func TestTranscribeTreatsAnAnswerWithoutTextAsBadResponse(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, `{}`))
	if _, err := h.client.Transcribe(context.Background(), TranscriptionRequest{OperationID: "k", Audio: sampleWebM}); !IsKind(err, KindBadResponse) {
		t.Fatalf("error = %v, want KindBadResponse", err)
	}
}

func TestTranscribeIsNeverRetried(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Status: http.StatusInternalServerError, Body: []byte(`{"error":{"type":"service_unavailable","message":"API dictation is unavailable."}}`)}, jsonBody(http.StatusOK, `{"text":"late"}`))
	if _, err := h.client.Transcribe(context.Background(), TranscriptionRequest{OperationID: "k", Audio: sampleWebM}); !IsKind(err, KindServer) {
		t.Fatalf("error = %v, want KindServer", err)
	}
	if n := len(h.http.Requests()); n != 1 {
		t.Fatalf("made %d requests, want 1", n)
	}
}

func TestPreparationPostsAndCancelDeletesTheOperation(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Status: http.StatusOK, Body: []byte(`{}`)}, fake.Response{Status: http.StatusOK, Body: []byte(`{}`)})
	if err := h.client.PrepareTranscription(context.Background(), "dictation-abc"); err != nil {
		t.Fatalf("PrepareTranscription: %v", err)
	}
	if err := h.client.CancelTranscriptionPreparation(context.Background(), "dictation-abc", true); err != nil {
		t.Fatalf("CancelTranscriptionPreparation: %v", err)
	}
	requests := h.http.Requests()
	if requests[0].Method != "POST" || requests[1].Method != "DELETE" {
		t.Fatalf("methods = %s, %s; want POST then DELETE", requests[0].Method, requests[1].Method)
	}
	if got := requests[1].Header.Get(cancellationReasonHdr); got != userCancelReason {
		t.Fatalf("cancellation reason = %q, want %s", got, userCancelReason)
	}
	for _, request := range requests {
		if request.URL != "https://api.nvm.fyi/api/v1/audio/transcription-preparations" || string(request.Body) != `{"operationId":"dictation-abc"}` {
			t.Fatalf("request = %s %s %s", request.Method, request.URL, request.Body)
		}
	}
}
