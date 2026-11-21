package chat

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestNoticeForMapsEveryFailureToItsOwnNotice checks each class of failure gets a notice that names it.
func TestNoticeForMapsEveryFailureToItsOwnNotice(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		partial string
		want    NoticeKind
	}{
		{"not signed in", ErrNotSignedIn, "", NoticeSignIn},
		{"unauthorized", &backend.Error{Kind: backend.KindUnauthorized, Status: 401}, "", NoticeSignIn},
		{"credits", &backend.Error{Kind: backend.KindInsufficientCredits, Status: 402}, "", NoticeCredits},
		{"spend cap", &backend.Error{Kind: backend.KindRejected, Status: 402, Code: "free_spend_cap_reached"}, "", NoticeSpendCap},
		{"rate limited", &backend.Error{Kind: backend.KindRateLimited, Status: 429, RetryAfter: time.Second}, "", NoticeRateLimited},
		{"too large", &backend.Error{Kind: backend.KindRejected, Status: 413, Code: "prompt_too_large"}, "", NoticeTooLarge},
		{"request shape", &backend.Error{Kind: backend.KindRejected, Status: 400, Code: "unsupported_request_shape"}, "", NoticeRequestShape},
		{"model unavailable", &backend.Error{Kind: backend.KindRejected, Status: 503, Code: "pricing_unavailable"}, "", NoticeModelUnavailable},
		{"update", &backend.Error{Kind: backend.KindIncompatible, Status: 426}, "", NoticeUpdate},
		{"offline", &backend.Error{Kind: backend.KindOffline}, "", NoticeOffline},
		{"timeout", &backend.Error{Kind: backend.KindTimeout}, "", NoticeTimeout},
		{"cut off", &backend.Error{Kind: backend.KindBadResponse}, "Hel", NoticeCutOff},
		{"failed without text", &backend.Error{Kind: backend.KindServer, Status: 500}, "", NoticeFailed},
		{"empty reply", ErrEmptyReply, "", NoticeFailed},
		{"unknown error", errors.New("boom"), "", NoticeFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notice := NoticeFor(tc.err, tc.partial)
			if notice.Kind != tc.want || notice.Title == "" || notice.Message == "" {
				t.Fatalf("notice = %+v, want kind %s with a title and copy", notice, tc.want)
			}
		})
	}
}

// TestPricingUnavailableUsesTheDesktopCopy checks a model that cannot be priced says what Electron says, whatever the
// server's message.
func TestPricingUnavailableUsesTheDesktopCopy(t *testing.T) {
	notice := NoticeFor(&backend.Error{Kind: backend.KindRejected, Status: 503, Code: "pricing_unavailable", Message: "server text"}, "")
	if notice.Title != "AI model unavailable" {
		t.Fatalf("title = %q, want the desktop title", notice.Title)
	}
	want := "Nevermind cannot price the selected AI model right now, so this request was not sent. Try again later or choose another model."
	if notice.Message != want {
		t.Fatalf("message = %q, want the desktop copy", notice.Message)
	}
}
