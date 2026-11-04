package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contractDir holds copies of the backend's recorded desktop-v1 contract, the bodies that src/backend/src/fixtures/
// contracts/desktop-v1 keeps and its route tests assert byte for byte. The copies live under testdata so the
// module tests run without the backend tree. Tests only; the client never reads these files at runtime.
const contractDir = "../../testdata/contracts/desktop-v1"

// readContract returns one recorded contract file.
func readContract(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractDir, name))
	if err != nil {
		t.Fatalf("read contract %s: %v", name, err)
	}
	return data
}

// TestRecordedManifestDecodesAndDrivesTheState checks the recorded compatibility manifest passes the required-field
// checks, and that a client behind the manifest'"'"'s latest release is not told to update, because that release
// belongs to the 1.x line.
func TestRecordedManifestDecodesAndDrivesTheState(t *testing.T) {
	var wire compatWire
	if err := json.Unmarshal(readContract(t, "compatibility-manifest.json"), &wire); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest, err := wire.manifest()
	if err != nil {
		t.Fatalf("manifest() rejected the recorded manifest: %v", err)
	}
	h := newHarness(t, WithClientVersion("0.6.1"))
	if state := h.client.deriveState(manifest); state != CompatOK {
		t.Fatalf("state = %q, want %q for 0.6.1 with the recorded manifest", state, CompatOK)
	}
	if manifest.Client.APIVersion != 1 || !manifest.Features["proxy_streaming"] {
		t.Fatalf("manifest fields = %+v", manifest)
	}
}

// TestRecordedDescriptorDecodesAndKeepsChatOff checks the recorded active-model descriptor decodes, and that its
// google-generative-ai wire format is reported as not usable by ChatStream.
func TestRecordedDescriptorDecodesAndKeepsChatOff(t *testing.T) {
	var descriptor ModelDescriptor
	if err := json.Unmarshal(readContract(t, "active-model-descriptor.json"), &descriptor); err != nil {
		t.Fatalf("decode descriptor: %v", err)
	}
	if descriptor.ID != "gemini-3-flash" || descriptor.CreditKind != "free" || descriptor.Credits == nil || descriptor.Credits.Total != 10 {
		t.Fatalf("descriptor = %+v", descriptor)
	}
	if descriptor.API != "google-generative-ai" || descriptor.SpeaksChatCompletions() {
		t.Fatalf("API %q must not be reported as chat-completions", descriptor.API)
	}
}

// TestRecordedDecisionDecodesEveryAnswerType checks the recorded decision response passes the desktop app's
// shape checks, and that a choice answer keeps the probabilities and confidence the model reported.
func TestRecordedDecisionDecodesEveryAnswerType(t *testing.T) {
	var wire decisionWire
	if err := json.Unmarshal(readContract(t, "decision-success.json"), &wire); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	result, err := wire.result()
	if err != nil {
		t.Fatalf("result() rejected the recorded decision: %v", err)
	}
	if result.Model != "typesafe/jev-1.13" || result.InputTokens != 120 || result.OutputTokens != 12 {
		t.Fatalf("result = %+v", result)
	}
	if answer := result.Answers["is_bug"]; answer.Type != "predicate" || !answer.Predicate || answer.Probability != 0.9 {
		t.Fatalf("predicate answer = %+v", answer)
	}
	if answer := result.Answers["area"]; answer.Type != "choice" || answer.Choice != "checkout" ||
		answer.Probabilities["checkout"] != 0.8 || answer.Probabilities["login"] != 0.2 ||
		answer.Confidence == nil || *answer.Confidence != 0.75 {
		t.Fatalf("choice answer = %+v", answer)
	}
	if answer := result.Answers["urgency"]; answer.Type != "score" || answer.Score != 3 {
		t.Fatalf("score answer = %+v", answer)
	}
}

// recordedError is one recorded error body, with the status the backend's route test asserts for it. The
// status is the one assertion the table can make from the record alone: where a route test does not assert a
// status, the table uses the status the error implies, and the type alone decides the kind.
type recordedError struct {
	file     string
	status   int
	wantKind Kind
	wantCode string
}

// TestRecordedErrorsClassifyByTypeAndStatus runs every recorded error through the client's classifier. The point
// is the unsupported_request_shape rows: that type names a refused request shape, not an outdated client, so it
// must never send the user to update.
func TestRecordedErrorsClassifyByTypeAndStatus(t *testing.T) {
	tests := []recordedError{
		{file: "device-auth-service-unavailable-error.json", status: 503, wantKind: KindServer, wantCode: "service_unavailable"},
		{file: "request-too-large-error.json", status: 413, wantKind: KindRejected, wantCode: codeRequestTooLarge},
		{file: "unsupported-request-shape-error.json", status: 400, wantKind: KindRejected, wantCode: codeUnsupportedRequestShape},
		{file: "unsupported-image-route-error.json", status: 400, wantKind: KindRejected, wantCode: codeUnsupportedRequestShape},
		{file: "unpriced-provider-tool-error.json", status: 400, wantKind: KindRejected, wantCode: codeUnsupportedRequestShape},
		{file: "decision-request-invalid-error.json", status: 400, wantKind: KindRejected, wantCode: codeInvalidDecisionRequest},
		{file: "decision-model-unavailable-error.json", status: 503, wantKind: KindServer, wantCode: codeDecisionModelUnavailable},
		{file: "decision-provider-response-invalid-error.json", status: 502, wantKind: KindServer, wantCode: codeDecisionProviderInvalid},
		{file: "pricing-unavailable-error.json", status: 503, wantKind: KindServer, wantCode: codePricingUnavailable},
		{file: "unsupported-client-error.json", status: 426, wantKind: KindIncompatible, wantCode: codeUnsupportedClient},
		{file: "unsupported-client-error.json", status: 400, wantKind: KindIncompatible, wantCode: codeUnsupportedClient},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			err := errorFromResponse(tt.status, "", readContract(t, tt.file))
			if err.Kind != tt.wantKind || err.Code != tt.wantCode {
				t.Fatalf("classified as kind %s code %q, want %s %q", err.Kind, err.Code, tt.wantKind, tt.wantCode)
			}
			if message := UserMessage(err); message == "" || message == copyUnknown || message == copyRejected {
				t.Fatalf("UserMessage = %q, want a specific line for %s", message, tt.wantCode)
			}
		})
	}
}

// TestRecordedUnsupportedClientNamesTheMinimumVersion checks the update line uses the version the backend sent.
func TestRecordedUnsupportedClientNamesTheMinimumVersion(t *testing.T) {
	err := errorFromResponse(426, "", readContract(t, "unsupported-client-error.json"))
	if err.MinimumVersion != "0.6.0" || err.UpdateURL != "https://example.com/update" {
		t.Fatalf("MinimumVersion = %q, UpdateURL = %q", err.MinimumVersion, err.UpdateURL)
	}
	if message := UserMessage(err); !strings.Contains(message, "0.6.0") {
		t.Fatalf("UserMessage = %q, want the minimum version", message)
	}
}
