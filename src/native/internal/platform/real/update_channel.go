package real

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// updateRepository is the GitHub repository ("owner/name") whose releases and release pointers hold the app's updates.
// Release builds link it (scripts/packaging/release-ldflags.sh). Without it the Nightly and Dev channels read MyGo's
// own feed with prereleases, which cannot tell a nightly from a dev build.
var updateRepository string

// The release lines, as the update channel names them.
const (
	updateChannelStable  = "stable"
	updateChannelNightly = "nightly"
	updateChannelDev     = "dev"
)

// maxChannelCandidates is how many releases a check tries, newest first, before it gives up: the pointer keeps the
// previous release as a fallback while the newest one's files are not published yet.
const maxChannelCandidates = 3

// maxManifestBytes is MyGo's limit for a manifest.
const maxManifestBytes = 1 << 20

var (
	devTagPattern     = regexp.MustCompile(`^v(\d+\.\d+\.\d+)-dev$`)
	nightlyTagPattern = regexp.MustCompile(`^v(\d+\.\d+\.\d+)-nightly$`)
	stableTagPattern  = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)
)

// errReleaseFileMissing is a release file that is not published (HTTP 404).
var errReleaseFileMissing = errors.New("not published")

// channelSource finds the update of a prerelease channel the way the release workflows publish it, as 1.x did: the
// dev and nightly builds are prereleases tagged v<version>-dev and v<version>-nightly, found through the small
// pointer files on the release-pointers branch, and Nightly also takes the latest stable release when it is newer.
// Stable needs no source: MyGo's feed is the repository's latest release.
type channelSource struct {
	channel    string
	repository string
	target     string
	// web and raw are github.com and raw.githubusercontent.com; tests replace them.
	web, raw string
	client   *http.Client
	now      func() time.Time
}

// channelRelease is a candidate release: its tag and the app version it holds.
type channelRelease struct {
	tag     string
	version string
}

// channelPointer is a pointer file: the channel's newest published tag first, then the previous one.
type channelPointer struct {
	Kind    string   `json:"kind"`
	Channel string   `json:"channel"`
	Tags    []string `json:"tags"`
}

// newChannelSource returns the source of channel for this build, or nil when MyGo's feed serves it.
func newChannelSource(channel string) mygo.UpdateSource {
	if updateRepository == "" || (channel != updateChannelNightly && channel != updateChannelDev) {
		return nil
	}
	return channelSource{
		channel: channel, repository: updateRepository, target: runtime.GOOS + "-" + runtime.GOARCH,
		web: "https://github.com", raw: "https://raw.githubusercontent.com", client: &http.Client{}, now: time.Now,
	}
}

// Manifest returns the manifest of the newest release on the channel that has one for this platform, and checks that
// it holds the version its tag names.
func (s channelSource) Manifest(ctx context.Context, _ bool) (io.ReadCloser, error) {
	tags, err := s.candidateTags(ctx)
	if err != nil {
		return nil, err
	}
	candidates := channelReleasesNewestFirst(tags, s.channel)
	for _, candidate := range candidates[:min(len(candidates), maxChannelCandidates)] {
		data, err := s.fetch(ctx, s.web+"/"+s.repository+"/releases/download/"+url.PathEscape(candidate.tag)+"/update-"+s.target+".json", maxManifestBytes, nil)
		if errors.Is(err, errReleaseFileMissing) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("reading the update of %s: %w", candidate.tag, err)
		}
		if manifest.Version != candidate.version {
			return nil, fmt.Errorf("the update of %s reports version %s", candidate.tag, manifest.Version)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("no %s release has published an update for %s", s.channel, s.target)
}

// Open downloads a file the manifest names; MyGo checks its size and signature.
func (s channelSource) Open(ctx context.Context, fileURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", fileURL, resp.Status)
	}
	return resp.Body, nil
}

// candidateTags are the pointer's tags, and on Nightly the latest stable release's tag after them.
func (s channelSource) candidateTags(ctx context.Context) ([]string, error) {
	var stable string
	if s.channel == updateChannelNightly {
		tag, err := s.latestStableTag(ctx)
		if err != nil {
			return nil, err
		}
		stable = tag
	}
	pointerURL := s.raw + "/" + s.repository + "/release-pointers/.channels/" + s.channel + ".json?check=" + strconv.FormatInt(s.now().UnixMilli(), 10)
	data, err := s.fetch(ctx, pointerURL, maxManifestBytes, http.Header{"Cache-Control": {"no-cache"}})
	if err != nil {
		return nil, fmt.Errorf("reading the %s release pointer: %w", s.channel, err)
	}
	var pointer channelPointer
	if err := json.Unmarshal(data, &pointer); err != nil || pointer.Kind != "nevermind-channel-pointer" || pointer.Channel != s.channel || len(pointer.Tags) > 20 {
		return nil, errors.New("invalid channel pointer")
	}
	if stable != "" {
		return append(pointer.Tags, stable), nil
	}
	return pointer.Tags, nil
}

// latestStableTag is the tag of the repository's latest release, or "" when there is none.
func (s channelSource) latestStableTag(ctx context.Context) (string, error) {
	data, err := s.fetch(ctx, s.web+"/"+s.repository+"/releases/latest", maxManifestBytes, http.Header{"Accept": {"application/json"}})
	if errors.Is(err, errReleaseFileMissing) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading the latest release: %w", err)
	}
	var latest struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &latest); err != nil {
		return "", fmt.Errorf("reading the latest release: %w", err)
	}
	return latest.TagName, nil
}

// fetch gets a small file, at most limit bytes, within 30 seconds.
func (s channelSource) fetch(ctx context.Context, fileURL string, limit int64, header http.Header) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errReleaseFileMissing
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GET %s: %s", fileURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", fileURL, limit)
	}
	return data, nil
}

// channelReleasesNewestFirst is 1.x's releasesNewestFirst: the releases of the tags that the channel offers, one per
// version, newest first. Dev offers only dev builds; Nightly offers nightlies and stable releases; a dev or nightly
// build holds the version of its tag without the suffix.
func channelReleasesNewestFirst(tags []string, channel string) []channelRelease {
	var releases []channelRelease
	for _, tag := range tags {
		version := ""
		switch {
		case channel == updateChannelDev:
			version = firstSubmatch(devTagPattern, tag)
		case channel == updateChannelNightly && nightlyTagPattern.MatchString(tag):
			version = firstSubmatch(nightlyTagPattern, tag)
		default:
			version = firstSubmatch(stableTagPattern, tag)
		}
		if version == "" || slices.ContainsFunc(releases, func(r channelRelease) bool { return r.version == version }) {
			continue
		}
		releases = append(releases, channelRelease{tag: tag, version: version})
	}
	slices.SortStableFunc(releases, func(a, b channelRelease) int { return compareReleaseVersions(b.version, a.version) })
	return releases
}

func firstSubmatch(pattern *regexp.Regexp, text string) string {
	if match := pattern.FindStringSubmatch(text); match != nil {
		return match[1]
	}
	return ""
}

// compareReleaseVersions orders two X.Y.Z versions numerically.
func compareReleaseVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(as), len(bs)) {
		an, _ := strconv.Atoi(as[i])
		bn, _ := strconv.Atoi(bs[i])
		if an != bn {
			return an - bn
		}
	}
	return len(as) - len(bs)
}
