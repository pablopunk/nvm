package fixtures

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestDeviceSignInIssuesApprovesAndUsesEachCodeOnce(t *testing.T) {
	backend := NewBackend()
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	ctx := context.Background()

	status, _, body := doThroughPort(t, set, ctx, http.MethodPost, DeviceInitiatePath, `{"label":"rig"}`)
	if status != http.StatusOK {
		t.Fatalf("initiate status %d body %q", status, body)
	}
	var initiate struct {
		Code      string `json:"code"`
		VerifyURL string `json:"verifyUrl"`
	}
	mustDecode(t, body, &initiate)
	if initiate.Code == "" || backend.LatestDeviceCode() != initiate.Code {
		t.Fatalf("initiate code %q, latest %q", initiate.Code, backend.LatestDeviceCode())
	}
	if !strings.HasPrefix(initiate.VerifyURL, "http://backend.test/auth/device?code=") {
		t.Fatalf("verification URL %q is not on the request host", initiate.VerifyURL)
	}

	exchangeBody := `{"code":"` + initiate.Code + `"}`
	if _, _, body := doThroughPort(t, set, ctx, http.MethodPost, DeviceExchangePath, exchangeBody); body != `{"status":"pending"}` {
		t.Fatalf("poll before approval: %q", body)
	}
	if err := backend.ApproveDeviceCode(initiate.Code); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := backend.ApproveDeviceCode(initiate.Code); !errors.Is(err, ErrDeviceCodeNotApprovable) {
		t.Fatalf("second approval error %v, want ErrDeviceCodeNotApprovable", err)
	}

	status, _, body = doThroughPort(t, set, ctx, http.MethodPost, DeviceExchangePath, exchangeBody)
	var approved struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		User   struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	mustDecode(t, body, &approved)
	if status != http.StatusOK || approved.Status != "ok" || approved.Token != FixtureAccount.Token || approved.User.Email != FixtureAccount.Email {
		t.Fatalf("approved poll: status %d body %q", status, body)
	}
	if status, _, _ := doThroughPort(t, set, ctx, http.MethodPost, DeviceExchangePath, exchangeBody); status != http.StatusGone {
		t.Fatalf("exchange after use status %d, want 410", status)
	}
}

func TestDeviceApprovalRouteRejectsUnknownCodes(t *testing.T) {
	backend := NewBackend()
	set := fake.NewSet()
	set.HTTP.Handle(backend)

	status, _, body := doThroughPort(t, set, context.Background(), http.MethodPost, DeviceApprovePath, `{"code":"NOPE"}`)
	if status != http.StatusBadRequest || body != InvalidDeviceCodeBody {
		t.Fatalf("approve unknown code: status %d body %q", status, body)
	}
	if status, _, _ := doThroughPort(t, set, context.Background(), http.MethodPost, DeviceExchangePath, `{"code":"NOPE"}`); status != http.StatusGone {
		t.Fatalf("exchange unknown code status %d, want 410", status)
	}
}

func TestTranscriptionAndPreparationRoutesAnswerTheirBodies(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Handle(NewBackend())
	ctx := context.Background()

	if status, _, body := doThroughPort(t, set, ctx, http.MethodPost, TranscriptionsPath, "webm"); status != http.StatusOK || body != TranscriptionBody {
		t.Fatalf("transcription: status %d body %q", status, body)
	}
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		if status, _, body := doThroughPort(t, set, ctx, method, TranscriptionPreparationPath, `{"operationId":"op"}`); status != http.StatusOK || body != OperationBody {
			t.Fatalf("%s preparation: status %d body %q", method, status, body)
		}
	}
}

func TestScriptedReplyIsStreamedInPlace(t *testing.T) {
	backend := NewBackend()
	backend.SetReply("Scripted reply.")
	set := fake.NewSet()
	set.HTTP.Handle(backend)

	_, _, body := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if !strings.Contains(body, `"content":"Scripted reply."`) || !strings.HasSuffix(body, DoneFrame) {
		t.Fatalf("scripted stream %q", body)
	}
}

func mustDecode(t *testing.T, body string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), into); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
}
