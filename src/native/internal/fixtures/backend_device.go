package fixtures

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Device sign-in routes of the fake backend. The real routes are under src/backend/src/pages/api/auth/device. The
// fake issues codes in order (FIXT-0001 onward) so a test can name them; the real codes are random. Initiate points
// the verification URL at the request's own host. Approval stands in for the browser session of the verification
// page: it is the only way a code becomes approved.

// Device sign-in routes.
const (
	DeviceInitiatePath = "/api/auth/device/initiate"
	DeviceExchangePath = "/api/auth/device/exchange"
	DeviceApprovePath  = "/api/auth/device/approve"
)

const (
	devicePollMillis   = 1000
	deviceCodeLifetime = 5 * time.Minute
)

// ErrDeviceCodeNotApprovable is returned when approval is asked for a code that is unknown, expired, already used
// or already approved.
var ErrDeviceCodeNotApprovable = errors.New("fixtures: the device code cannot be approved")

// FixtureAccount is the account a device sign-in approves and the token it receives. Both are fixture values, not
// credentials. The email uses the reserved example.test domain.
var FixtureAccount = DeviceAccount{
	ID:    "user_fixture",
	Email: "fixture@example.test",
	Role:  "member",
	Token: "fixture-device-token",
}

// DeviceAccount is the account a device sign-in approves.
type DeviceAccount struct {
	ID    string
	Email string
	Role  string
	Token string
}

// deviceCode is one issued code: when it stops working and whether it was approved or exchanged already.
type deviceCode struct {
	expiresAt time.Time
	approved  bool
	consumed  bool
}

// deviceExchange is the answer of one poll of a code.
type deviceExchange int

const (
	exchangePending deviceExchange = iota
	exchangeApproved
	exchangeExpired
)

// deviceSignIns holds the codes the fake has issued.
type deviceSignIns struct {
	mu     sync.Mutex
	codes  map[string]*deviceCode
	issued int
	latest string
}

func newDeviceSignIns() *deviceSignIns {
	return &deviceSignIns{codes: map[string]*deviceCode{}}
}

// issue starts a sign-in at now and returns its code and when the code expires.
func (d *deviceSignIns) issue(now time.Time) (string, time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.issued++
	code := fmt.Sprintf("FIXT-%04d", d.issued)
	expiresAt := now.Add(deviceCodeLifetime)
	d.codes[code] = &deviceCode{expiresAt: expiresAt}
	d.latest = code
	return code, expiresAt
}

// approve marks a pending code approved at now.
func (d *deviceSignIns) approve(code string, now time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.live(code, now)
	if !ok || entry.approved {
		return ErrDeviceCodeNotApprovable
	}
	entry.approved = true
	return nil
}

// exchange answers one poll of code at now. An approved code is used up by the exchange, as the backend does.
func (d *deviceSignIns) exchange(code string, now time.Time) deviceExchange {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.live(code, now)
	switch {
	case !ok:
		return exchangeExpired
	case !entry.approved:
		return exchangePending
	}
	entry.consumed = true
	return exchangeApproved
}

// live returns the entry of code when it is known, not expired and not used. The caller holds d.mu.
func (d *deviceSignIns) live(code string, now time.Time) (*deviceCode, bool) {
	entry, ok := d.codes[code]
	if !ok || entry.consumed || !now.Before(entry.expiresAt) {
		return nil, false
	}
	return entry, true
}

// latestCode returns the most recently issued code, or "" before the first sign-in.
func (d *deviceSignIns) latestCode() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.latest
}

// ApproveDeviceCode approves a pending device code, as the approving browser session would. It fails for codes that
// are unknown, expired, used or already approved.
func (b *Backend) ApproveDeviceCode(code string) error {
	return b.device.approve(code, b.clock.Now())
}

// LatestDeviceCode returns the code of the most recent sign-in, or "" when none has started.
func (b *Backend) LatestDeviceCode() string {
	return b.device.latestCode()
}

func (b *Backend) serveDeviceInitiate(w http.ResponseWriter, r *http.Request) {
	code, expiresAt := b.device.issue(b.clock.Now())
	verifyURL := "http://" + r.Host + "/auth/device?code=" + url.QueryEscape(code)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":           code,
		"verifyUrl":      verifyURL,
		"expiresAt":      expiresAt.Format(time.RFC3339Nano),
		"pollIntervalMs": devicePollMillis,
	})
}

func (b *Backend) serveDeviceExchange(w http.ResponseWriter, body string) {
	code, ok := codeFromBody(body)
	if !ok {
		writeBody(w, http.StatusBadRequest, "application/json", InvalidRequestBody)
		return
	}
	switch b.device.exchange(code, b.clock.Now()) {
	case exchangePending:
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
	case exchangeApproved:
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"token":  FixtureAccount.Token,
			"user":   map[string]string{"id": FixtureAccount.ID, "email": FixtureAccount.Email, "role": FixtureAccount.Role},
		})
	default:
		writeJSON(w, http.StatusGone, map[string]string{"status": "expired"})
	}
}

func (b *Backend) serveDeviceApprove(w http.ResponseWriter, body string) {
	code, ok := codeFromBody(body)
	if !ok || b.ApproveDeviceCode(code) != nil {
		writeBody(w, http.StatusBadRequest, "text/plain; charset=utf-8", InvalidDeviceCodeBody)
		return
	}
	writeBody(w, http.StatusOK, "application/json", OperationBody)
}

// codeFromBody reads the code field of a device request body.
func codeFromBody(body string) (string, bool) {
	var request struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil || request.Code == "" {
		return "", false
	}
	return request.Code, true
}

// writeJSON encodes value and writes it with status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		panic("fixtures: encoding a device response failed: " + err.Error())
	}
	writeBody(w, status, "application/json", string(data))
}
