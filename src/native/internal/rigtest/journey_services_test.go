//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// servicesRows maps each query the acme.services fixture answers to the exact row the host shows for it. The
// network row proves a loopback address is refused by the public port. The secrets query is not asserted here: the
// box may have no secret service, and then the row reports unavailable instead of a value.
var servicesRows = []struct{ query, label string }{
	{"storage", "storage found=true city=Lisbon"},
	{"memo", "memo value=42 again=42 loads=1"},
	{"settings", "settings default=metric definitions=2 compact=true"},
	{"logs", "logs recent=1"},
	{"text", "text=hi there cursor=8"},
	{"network", "network permission=true"},
	{"cache", "cache found=true value=7"},
	{"shared", "shared written"},
}

// TestJourneyServicesFixtureStoresAndFetchesThroughTheHost enables the services fixture through its review and
// checks that each context service answers the way the host should: storage and memo round trips, typed settings,
// logs stamped with the extension, text expansion, the public-address refusal, the process cache and a plain write.
func TestJourneyServicesFixtureStoresAndFetchesThroughTheHost(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.services")
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	clearSearch(t, app)
	enableThroughReview(t, app, "services", "Services fixture")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	started := time.Now()
	for _, check := range servicesRows {
		clearSearch(t, app)
		must(t, app.Type(check.query))
		waitForRow(t, app, check.label, elementWait)
	}
	t.Logf("all %d service rows shown within %s", len(servicesRows), time.Since(started).Round(time.Millisecond))
	shoot(t, app, "services-fixture-rows")
}
