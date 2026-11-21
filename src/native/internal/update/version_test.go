package update

import "testing"

func TestCompareFollowsSemverForTheVersionsTheAppShips(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.0.0", "2.0.0", 0},
		{"2.10.0", "2.9.0", 1},
		{"2.0.0-next.0", "2.0.0-next.1", -1},
		{"2.0.0-next.10", "2.0.0-next.9", 1},
		{"2.0.0-next.1", "2.0.0", -1},
		{"2.0.0", "2.0.0-next.1", 1},
		{"v2.1.0", "2.0.9", 1},
		{"1.9.9", "2.0.0", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewerRefusesSameAndOlderVersions(t *testing.T) {
	if !IsNewer("2.0.1", "2.0.0") {
		t.Error("2.0.1 is not newer than 2.0.0")
	}
	for _, older := range []string{"2.0.0", "1.9.0", "2.0.0-next.3"} {
		if IsNewer(older, "2.0.0") {
			t.Errorf("%s offered as newer than 2.0.0", older)
		}
	}
}

func TestChannelsParseLabelAndCycle(t *testing.T) {
	if ParseChannel("nightly") != ChannelNightly || ParseChannel("dev") != ChannelDev || ParseChannel("") != ChannelStable {
		t.Fatal("stored channels must parse, unknown values fall back to stable")
	}
	if ChannelStable.Next() != ChannelNightly || ChannelNightly.Next() != ChannelDev || ChannelDev.Next() != ChannelStable {
		t.Fatal("Next must cycle Stable, Nightly, Dev, as 1.x lists the channels")
	}
	if ChannelStable.Prereleases() || !ChannelNightly.Prereleases() || !ChannelDev.Prereleases() {
		t.Fatal("only nightly and dev offer prereleases")
	}
	if ChannelStable.Label() != "Stable" || ChannelNightly.Label() != "Nightly" || ChannelDev.Label() != "Dev" {
		t.Fatal("channel labels changed")
	}
}

func TestMessagesNameTheStageAndTheProgress(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  string
	}{
		{"downloading", State{Status: StatusDownloading, LatestVersion: "2.1.0", Progress: 0.426}, "Downloading Nevermind 2.1.0 (43%)"},
		{"ready", State{Status: StatusDownloaded, LatestVersion: "2.1.0"}, "Nevermind 2.1.0 is ready. Restart to finish updating."},
		{"install failed", State{Status: StatusFailed, Stage: StageInstall, Message: "not signed"}, "Update install failed: not signed"},
		{"check failed", State{Status: StatusFailed, Stage: StageCheck, Message: "offline"}, "Update check failed: offline"},
		{"checking", State{Status: StatusChecking}, "Checking for updates"},
	}
	for _, c := range cases {
		if got := Message(c.state); got != c.want {
			t.Errorf("%s: Message = %q, want %q", c.name, got, c.want)
		}
	}
}
