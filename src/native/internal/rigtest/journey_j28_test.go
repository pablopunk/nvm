//go:build rig

package rigtest

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// J28 (PLAN.md 3.1): a refusal from the backend ends the turn with its own notice. A 402 names the credits the request
// needed and the balance; a route the free pool has paused gets the spend-cap notice. The 402 comes from the route
// lookup that starts every turn. Dismissing the palette from the chat page after a notice is not covered yet. Offline,
// expired tokens and rate limits are not covered yet.
const (
	creditsNoticeTitle  = "Credits needed"
	creditsNoticeDetail = "your balance is 81"
	spendCapNoticeTitle = "Free AI temporarily paused"
	pausedRouteBody     = `{"id":"fixture-route","name":"Fixture Route","contextWindow":128000,"maxInputTokens":100000,` +
		`"maxTokens":4096,"reasoning":false,"thinkingLevel":"off","input":["text"],"api":"openai-completions",` +
		`"provider":"fixture","baseUrl":"https://fixture.test","modelTier":"free","creditKind":"free",` +
		`"credits":{"paid":0,"free":0,"total":0},"notice":"blocked","noticeReason":"free_spend_cap_reached",` +
		`"costEstimate":null}`
)

func TestJourneyJ28RefusalsShowTheirNotices(t *testing.T) {
	t.Run("insufficient credits", func(t *testing.T) {
		fake := StartFakeBackend(t)
		app := Launch(t, Options{Backend: fake.URL})
		signInThroughAccount(t, app, fake)
		hidePalette(t, app)
		must(t, app.SummonPalette())

		fake.SetMode(fixtures.ModeInsufficientCredits)
		askAIFromRoot(t, app, aiQuestion)
		_, err := app.WaitFor(creditsNoticeTitle, elementWait)
		must(t, err)
		_, err = app.WaitFor(creditsNoticeDetail, elementWait)
		must(t, err)
		shoot(t, app, "j28-1-credits")
	})

	t.Run("free pool paused", func(t *testing.T) {
		fake := StartFakeBackend(t)
		app := Launch(t, Options{Backend: fake.URL})
		signInThroughAccount(t, app, fake)
		hidePalette(t, app)
		must(t, app.SummonPalette())

		fake.SetActiveModel(pausedRouteBody)
		askAIFromRoot(t, app, aiQuestion)
		_, err := app.WaitFor(spendCapNoticeTitle, elementWait)
		must(t, err)
		shoot(t, app, "j28-2-spend-cap")
	})
}
