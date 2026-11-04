package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// customOrigin is a preview backend the environment tests switch to.
const customOrigin = "https://pr-42.preview.example"

// TestSwitchingBackendKeepsEachOriginsToken checks that switching away and back does not sign the user out.
func TestSwitchingBackendKeepsEachOriginsToken(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	if err := r.session.SwitchEnvironment(backend.EnvironmentCustom, customOrigin); err != nil {
		t.Fatalf("SwitchEnvironment(custom): %v", err)
	}
	away := r.session.Snapshot()
	if away.Connected || away.BaseURL != customOrigin || away.Environment != backend.EnvironmentCustom {
		t.Fatalf("on the preview backend = %+v, want not connected at %s", away, customOrigin)
	}
	if err := r.session.SwitchEnvironment(backend.EnvironmentProduction, ""); err != nil {
		t.Fatalf("SwitchEnvironment(production): %v", err)
	}
	back := r.session.Snapshot()
	if !back.Connected || back.Email != "pat@example.test" || back.BaseURL != productionOrigin {
		t.Fatalf("back on production = %+v, want the connected account", back)
	}
}

// TestEnvironmentChoiceSurvivesARestart checks the active backend is read back from the account document.
func TestEnvironmentChoiceSurvivesARestart(t *testing.T) {
	r := newRig(t, rigOptions{})
	if err := r.session.SwitchEnvironment(backend.EnvironmentCustom, customOrigin); err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
	restarted := r.reopen().Snapshot()
	if restarted.BaseURL != customOrigin || restarted.Environment != backend.EnvironmentCustom {
		t.Fatalf("after restart = %+v, want the preview backend", restarted)
	}
}

// TestPackagedBuildRefusesDevelopmentAndPrivateOrigins checks the environment rules of a packaged build.
func TestPackagedBuildRefusesDevelopmentAndPrivateOrigins(t *testing.T) {
	r := newRig(t, rigOptions{packaged: true})
	if err := r.session.SwitchEnvironment(backend.EnvironmentDevelopment, ""); err == nil {
		t.Fatal("a packaged build switched to development")
	}
	if err := r.session.SwitchEnvironment(backend.EnvironmentCustom, "https://10.0.0.5"); err == nil {
		t.Fatal("a packaged build switched to a private-network origin")
	}
	if got := r.session.Snapshot().BaseURL; got != productionOrigin {
		t.Fatalf("base URL after refusals = %s, want production", got)
	}
}

// TestSwitchingBackendStopsARunningSignIn checks a sign-in does not finish against the backend the user left.
func TestSwitchingBackendStopsARunningSignIn(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	r.waitForPhase(PhasePending)
	if err := r.session.SwitchEnvironment(backend.EnvironmentCustom, customOrigin); err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
	if phase := r.session.Snapshot().SignIn.Phase; phase != PhaseIdle {
		t.Fatalf("phase after switch = %s, want idle", phase)
	}
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("a new sign-in after the switch was refused: %v", err)
	}
	r.session.CancelSignIn()
}

// TestSignOutRevokesWithTheTokenThenForgetsTheAccount checks the order: the revocation carries the bearer, and the
// token and the account are removed only after it.
func TestSignOutRevokesWithTheTokenThenForgetsTheAccount(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	r.set.HTTP.Script(fake.Response{Status: http.StatusNoContent})

	if revoked := r.session.SignOut(context.Background()); !revoked {
		t.Fatal("SignOut reported the revocation as failed")
	}
	requests := r.set.HTTP.Requests()
	last := requests[len(requests)-1]
	if last.Method != http.MethodDelete || last.URL != productionOrigin+"/api/tokens/current" {
		t.Fatalf("revocation request = %s %s", last.Method, last.URL)
	}
	if got := last.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Fatalf("revocation sent %q, want the bearer token", got)
	}
	if _, ok := r.storedKeychainToken(productionAccount); ok {
		t.Fatal("the token is still in the keychain after sign-out")
	}
	if snapshot := r.session.Snapshot(); snapshot.Connected || snapshot.Email != "" {
		t.Fatalf("after sign-out = %+v, want signed out", snapshot)
	}
	r.flush()
	document, _ := r.set.FS.ReadFile("/config/account.json")
	if strings.Contains(string(document), "pat@example.test") {
		t.Fatal("the account document still names the signed-out account")
	}
}

// TestSignOutClearsLocallyWhenTheBackendRefuses checks a failed revocation is reported without keeping the token.
func TestSignOutClearsLocallyWhenTheBackendRefuses(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	r.set.HTTP.Script(fake.Response{Status: http.StatusInternalServerError, Body: []byte("boom")})
	if revoked := r.session.SignOut(context.Background()); revoked {
		t.Fatal("SignOut reported a failed revocation as revoked")
	}
	if _, ok := r.storedKeychainToken(productionAccount); ok {
		t.Fatal("the token survived a sign-out")
	}
	if r.session.Snapshot().Connected {
		t.Fatal("still connected after sign-out")
	}
}

// TestRefreshReadsBalanceSpendCapAndPlan checks the account view's numbers come from the backend's answers.
func TestRefreshReadsBalanceSpendCapAndPlan(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	r.set.HTTP.Script(
		jsonResponse(200, `{"id":"fast","name":"Fast","api":"openai-completions","notice":"blocked","noticeReason":"free_spend_cap_reached","credits":{"paid":0,"free":0,"total":1200}}`),
		jsonResponse(200, `{"email":"pat@example.test","plan":"pro","balance":1200}`),
	)
	if err := r.session.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	snapshot := r.session.Snapshot()
	if !snapshot.Account.HasBalance || snapshot.Account.Balance != 1200 || snapshot.Account.Plan != "pro" {
		t.Fatalf("account = %+v, want balance 1200 on plan pro", snapshot.Account)
	}
	if snapshot.Notice == nil || snapshot.Notice.Kind != freeSpendCapCode {
		t.Fatalf("notice = %+v, want the spend-cap notice", snapshot.Notice)
	}
}

// TestRefreshToleratesAPlanTheBackendWillNotGive checks that a refused account read does not hide the balance.
func TestRefreshToleratesAPlanTheBackendWillNotGive(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	r.set.HTTP.Script(
		jsonResponse(200, `{"id":"fast","name":"Fast","api":"openai-completions","credits":{"paid":5,"free":0,"total":5}}`),
		jsonResponse(http.StatusUnauthorized, "Unauthorized"),
	)
	if err := r.session.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	account := r.session.Snapshot().Account
	if account.Plan != "" || account.Error != "" || account.Balance != 5 {
		t.Fatalf("account = %+v, want the balance with no plan and no error", account)
	}
}

// TestRefreshReportsAnUnreachableBackend checks the account view is told the details could not be read.
func TestRefreshReportsAnUnreachableBackend(t *testing.T) {
	r := newRig(t, rigOptions{clientOptions: []backend.Option{backend.WithRetryPolicy(1, 0, 0)}})
	r.signIn()
	r.set.HTTP.Script(fake.Response{Err: errors.New("no route to host")}, fake.Response{Err: errors.New("no route to host")})
	if err := r.session.Refresh(context.Background()); !backend.IsKind(err, backend.KindOffline) {
		t.Fatalf("Refresh error = %v, want KindOffline", err)
	}
	if r.session.Snapshot().Account.Error == "" {
		t.Fatal("the view was not told the account details are unavailable")
	}
}

// TestBYOKeyStaysInTheKeychain checks the key is stored, read back and cleared, and never written to the account
// document or the logs.
func TestBYOKeyStaysInTheKeychain(t *testing.T) {
	const key = "sk-byo-canary-991"
	r := newRig(t, rigOptions{})
	input := BYOInput{Provider: "openai", ModelID: "example-model", BaseURL: "https://api.openai.example/v1/", APIKey: key}
	if err := r.session.SetBYOKey(input); err != nil {
		t.Fatalf("SetBYOKey: %v", err)
	}
	setting := r.session.Snapshot().BYO
	if setting == nil || setting.Provider != "openai" || setting.BaseURL != "https://api.openai.example/v1" {
		t.Fatalf("BYO = %+v, want the provider with a trimmed base URL", setting)
	}
	if got, ok := r.session.BYOKey(); !ok || got != key {
		t.Fatalf("BYOKey = %q, %v; want the stored key", got, ok)
	}

	r.flush()
	document, _ := r.set.FS.ReadFile("/config/account.json")
	surfaces := map[string]string{"account.json": string(document), "logs": r.logText()}
	for name, text := range surfaces {
		if strings.Contains(text, key) {
			t.Fatalf("%s contains the BYO key", name)
		}
	}

	if err := r.session.ClearBYOKey(); err != nil {
		t.Fatalf("ClearBYOKey: %v", err)
	}
	if _, ok := r.session.BYOKey(); ok || r.session.Snapshot().BYO != nil {
		t.Fatal("the key or its setting survived ClearBYOKey")
	}
}

// TestBYOKeyNeedsAUsableProvider checks the fields a provider request cannot do without, and the origin rule.
func TestBYOKeyNeedsAUsableProvider(t *testing.T) {
	valid := BYOInput{Provider: "openai", ModelID: "m", BaseURL: "https://api.example.test/v1", APIKey: "k"}
	cases := map[string]func(*BYOInput){
		"missing provider":          func(i *BYOInput) { i.Provider = " " },
		"missing model":             func(i *BYOInput) { i.ModelID = "" },
		"missing key":               func(i *BYOInput) { i.APIKey = "  " },
		"remote plain http":         func(i *BYOInput) { i.BaseURL = "http://api.example.test/v1" },
		"not an absolute URL":       func(i *BYOInput) { i.BaseURL = "api.example.test" },
		"unrecognized scheme on it": func(i *BYOInput) { i.BaseURL = "ftp://api.example.test" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if err := validateBYO(input); err == nil {
				t.Fatalf("validateBYO(%+v) accepted the input", input)
			}
		})
	}
	local := valid
	local.BaseURL = "http://localhost:11434/v1"
	if err := validateBYO(local); err != nil {
		t.Fatalf("a loopback provider was refused: %v", err)
	}
}

// TestSpendCapNoticeFromRefusalsAndRoutes checks the notice shows for both signals the backend sends, and carries
// the reset time when the refusal names one.
func TestSpendCapNoticeFromRefusalsAndRoutes(t *testing.T) {
	refusal := &backend.Error{Kind: backend.KindInsufficientCredits, Status: 402, Code: freeSpendCapCode, Message: `{"resetsAt":"2026-12-01T00:00:00Z"}`}
	notice, ok := SpendCapNoticeFromError(fmt.Errorf("chat: %w", refusal))
	if !ok || notice.ResetsAt != "2026-12-01T00:00:00Z" || notice.Title != "Free AI temporarily paused" {
		t.Fatalf("notice from refusal = %+v, %v; want the titled notice with the reset time", notice, ok)
	}
	if !strings.Contains(notice.Message, "resets at 2026-12-01T00:00:00Z") {
		t.Fatalf("message = %q, want the reset time", notice.Message)
	}
	if _, ok := SpendCapNoticeFromError(&backend.Error{Kind: backend.KindRejected, Code: "other"}); ok {
		t.Fatal("an unrelated refusal produced a spend-cap notice")
	}
	blocked := backend.ModelDescriptor{Notice: blockedNotice, NoticeReason: freeSpendCapCode}
	if _, ok := SpendCapNoticeFromModel(blocked); !ok {
		t.Fatal("a blocked route produced no notice")
	}
	if _, ok := SpendCapNoticeFromModel(backend.ModelDescriptor{Notice: "ok"}); ok {
		t.Fatal("an unblocked route produced a notice")
	}
}

// TestNoteErrorKeepsTheSpendCapNoticeAcrossOtherErrors checks a later unrelated error does not hide the notice.
func TestNoteErrorKeepsTheSpendCapNoticeAcrossOtherErrors(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.session.NoteError(&backend.Error{Kind: backend.KindInsufficientCredits, Code: freeSpendCapCode})
	r.session.NoteError(errors.New("network down"))
	if r.session.Snapshot().Notice == nil {
		t.Fatal("an unrelated error cleared the spend-cap notice")
	}
}
