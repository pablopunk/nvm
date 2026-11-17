package real

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// releaseHost serves a repository's latest release, its release pointers and the manifests of its releases.
type releaseHost struct {
	latest    string
	pointers  map[string]string
	manifests map[string]string
	requests  []string
}

func (h *releaseHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.requests = append(h.requests, r.URL.Path)
	switch {
	case r.URL.Path == "/me/app/releases/latest" && h.latest != "":
		_, _ = io.WriteString(w, `{"tag_name":"`+h.latest+`"}`)
	case strings.HasPrefix(r.URL.Path, "/me/app/release-pointers/.channels/"):
		channel := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/me/app/release-pointers/.channels/"), ".json")
		if r.Header.Get("Cache-Control") != "no-cache" || r.URL.Query().Get("check") == "" {
			http.Error(w, "the pointer is read past caches", http.StatusBadRequest)
			return
		}
		if pointer, ok := h.pointers[channel]; ok {
			_, _ = io.WriteString(w, pointer)
			return
		}
		http.NotFound(w, r)
	case strings.HasPrefix(r.URL.Path, "/me/app/releases/download/"):
		if manifest, ok := h.manifests[strings.TrimPrefix(r.URL.Path, "/me/app/releases/download/")]; ok {
			_, _ = io.WriteString(w, manifest)
			return
		}
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

func testChannelSource(t *testing.T, channel string, host *releaseHost) channelSource {
	t.Helper()
	server := httptest.NewServer(host)
	t.Cleanup(server.Close)
	return channelSource{
		channel: channel, repository: "me/app", target: "linux-amd64", web: server.URL, raw: server.URL,
		client: server.Client(), now: func() time.Time { return time.Unix(1700000000, 0) },
	}
}

func readManifestVersion(t *testing.T, source channelSource) string {
	t.Helper()
	body, err := source.Manifest(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	return string(data)
}

func manifestOf(version string) string {
	return `{"version":"` + version + `","url":"u","signature":"s"}`
}

func nightlyPointer(tags ...string) string {
	return `{"kind":"nevermind-channel-pointer","channel":"nightly","tags":["` + strings.Join(tags, `","`) + `"]}`
}

func TestNightlyTakesTheNewerOfThePointedNightlyAndTheLatestStable(t *testing.T) {
	host := &releaseHost{
		latest:   "v2026.1110.1",
		pointers: map[string]string{"nightly": nightlyPointer("v2026.1117.2-nightly", "v2026.1116.1-nightly")},
		manifests: map[string]string{
			"v2026.1117.2-nightly/update-linux-amd64.json": manifestOf("2026.1117.2"),
			"v2026.1110.1/update-linux-amd64.json":         manifestOf("2026.1110.1"),
		},
	}
	if got := readManifestVersion(t, testChannelSource(t, "nightly", host)); got != manifestOf("2026.1117.2") {
		t.Fatalf("manifest = %s", got)
	}
	host.latest = "v2026.1118.1"
	host.manifests["v2026.1118.1/update-linux-amd64.json"] = manifestOf("2026.1118.1")
	if got := readManifestVersion(t, testChannelSource(t, "nightly", host)); got != manifestOf("2026.1118.1") {
		t.Fatalf("a newer stable release must win on Nightly, manifest = %s", got)
	}
}

// A release whose manifest is not published yet, such as one from before this app, falls back to the next one.
func TestNightlySkipsAReleaseWithoutAManifestForThisPlatform(t *testing.T) {
	host := &releaseHost{
		pointers:  map[string]string{"nightly": nightlyPointer("v2026.1117.2-nightly", "v2026.1116.1-nightly")},
		manifests: map[string]string{"v2026.1116.1-nightly/update-linux-amd64.json": manifestOf("2026.1116.1")},
	}
	if got := readManifestVersion(t, testChannelSource(t, "nightly", host)); got != manifestOf("2026.1116.1") {
		t.Fatalf("manifest = %s", got)
	}
}

func TestDevFollowsOnlyTheDevPointer(t *testing.T) {
	host := &releaseHost{
		latest:   "v2026.1120.1",
		pointers: map[string]string{"dev": `{"kind":"nevermind-channel-pointer","channel":"dev","tags":["v2026.1117.3-dev"]}`},
		manifests: map[string]string{
			"v2026.1117.3-dev/update-linux-amd64.json": manifestOf("2026.1117.3"),
			"v2026.1120.1/update-linux-amd64.json":     manifestOf("2026.1120.1"),
		},
	}
	if got := readManifestVersion(t, testChannelSource(t, "dev", host)); got != manifestOf("2026.1117.3") {
		t.Fatalf("manifest = %s", got)
	}
	if slices.Contains(host.requests, "/me/app/releases/latest") {
		t.Fatal("the dev channel read the latest stable release")
	}
}

func TestAManifestOfAnotherVersionThanItsTagIsRefused(t *testing.T) {
	host := &releaseHost{
		pointers:  map[string]string{"nightly": nightlyPointer("v2026.1117.2-nightly")},
		manifests: map[string]string{"v2026.1117.2-nightly/update-linux-amd64.json": manifestOf("2026.1117.1")},
	}
	if _, err := testChannelSource(t, "nightly", host).Manifest(context.Background(), false); err == nil {
		t.Fatal("a manifest that does not match its tag was accepted")
	}
}

func TestAPointerOfAnotherChannelIsRefused(t *testing.T) {
	host := &releaseHost{pointers: map[string]string{"dev": nightlyPointer("v2026.1117.2-nightly")}}
	if _, err := testChannelSource(t, "dev", host).Manifest(context.Background(), false); err == nil {
		t.Fatal("a nightly pointer was read as the dev pointer")
	}
}

func TestChannelReleasesNewestFirstKeepsEachChannelsTags(t *testing.T) {
	tags := []string{"v2026.1117.2-nightly", "v2026.1117.5-dev", "v2026.1110.1", "v2026.1117.2", "v2.0.0-next.1"}
	var nightly []string
	for _, r := range channelReleasesNewestFirst(tags, "nightly") {
		nightly = append(nightly, r.tag)
	}
	if want := []string{"v2026.1117.2-nightly", "v2026.1110.1"}; !slices.Equal(nightly, want) {
		t.Fatalf("nightly = %v, want %v", nightly, want)
	}
	dev := channelReleasesNewestFirst(tags, "dev")
	if len(dev) != 1 || dev[0].tag != "v2026.1117.5-dev" || dev[0].version != "2026.1117.5" {
		t.Fatalf("dev = %+v", dev)
	}
}

// Without a linked repository, Stable and the prerelease channels read MyGo's own feed.
func TestNoChannelSourceWithoutARepositoryOrOnStable(t *testing.T) {
	if newChannelSource("nightly") != nil {
		t.Fatal("an unlinked build made a channel source")
	}
	updateRepository = "me/app"
	t.Cleanup(func() { updateRepository = "" })
	if newChannelSource("stable") != nil || newChannelSource("nightly") == nil || newChannelSource("dev") == nil {
		t.Fatal("only Nightly and Dev read the release pointers")
	}
}
