package auth

import (
	"errors"
	"regexp"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// freeSpendCapCode is the backend's reason for pausing the free credit pool.
const freeSpendCapCode = "free_spend_cap_reached"

// blockedNotice is the notice value a model route carries when the cap blocks it.
const blockedNotice = "blocked"

// resetsAtPattern finds the reset time the backend may put in a refusal body.
var resetsAtPattern = regexp.MustCompile(`"resetsAt"\s*:\s*"([^"]+)"`)

// Notice is a message the account view shows above its rows.
type Notice struct {
	// Kind is the backend reason the notice explains.
	Kind string
	// Title is the heading of the notice.
	Title string
	// Message explains the notice in plain words.
	Message string
	// ResetsAt is when the free pool resets, when the backend said so.
	ResetsAt string
}

// SpendCapNoticeFromError returns the notice for a request the backend refused because the free pool is paused.
func SpendCapNoticeFromError(err error) (Notice, bool) {
	var typed *backend.Error
	if !errors.As(err, &typed) || typed.Code != freeSpendCapCode {
		return Notice{}, false
	}
	resetsAt := ""
	if match := resetsAtPattern.FindStringSubmatch(typed.Message); match != nil {
		resetsAt = match[1]
	}
	return freeSpendCapNotice(resetsAt), true
}

// SpendCapNoticeFromModel returns the notice for a route the backend reports as blocked by the free pool's cap.
func SpendCapNoticeFromModel(descriptor backend.ModelDescriptor) (Notice, bool) {
	if descriptor.Notice != blockedNotice || descriptor.NoticeReason != freeSpendCapCode {
		return Notice{}, false
	}
	return freeSpendCapNotice(""), true
}

// freeSpendCapNotice builds the notice text, with the reset time when there is one.
func freeSpendCapNotice(resetsAt string) Notice {
	message := "Free-pool requests are temporarily paused. Capacity may return earlier as in-flight reservations settle or release."
	if resetsAt != "" {
		message += " The current period resets at " + resetsAt + "."
	}
	return Notice{
		Kind:     freeSpendCapCode,
		Title:    "Free AI temporarily paused",
		Message:  message,
		ResetsAt: resetsAt,
	}
}
