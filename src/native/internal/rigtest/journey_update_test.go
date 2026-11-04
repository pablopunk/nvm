//go:build rig

package rigtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The update journeys run a real app built with a fake update feed on loopback and a fake signing key. The key is
// the rig's own (its seed is written here); no release is signed with it. scripts/rig/update-journey.sh builds the
// two binaries and links the key's public half and the feed's address into them.
const (
	rigUpdateSeedText = "nevermind rig update fake key (not a release key)"
	rigUpdateFile     = "nevermind-next"
	rigUpdateManifest = "/update-linux-amd64.json"
	rigUpdateArchive  = "/update-2.0.1.tar.gz"
	rigUpdateWait     = 150 * time.Second
	rigUpdatePortEnv  = "NVM_RIG_UPDATE_PORT"
	rigUpdateDefault  = "18765"
)

// rigUpdateKey is the fake Ed25519 private key the feed signs with.
func rigUpdateKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(rigUpdateSeedText))
	return ed25519.NewKeyFromSeed(seed[:])
}

// updateFeed serves a manifest and an archive of the new build, as a release feed does. Tamper signs the archive with
// a different key, which the app must refuse.
type updateFeed struct {
	mu       sync.Mutex
	manifest []byte
	archive  []byte
	served   map[string]int
}

func newUpdateFeed(t *testing.T, newBinary string, tamper bool) *updateFeed {
	t.Helper()
	binary, err := os.ReadFile(newBinary)
	must(t, err)
	archive := tarGzipBinary(t, binary)
	digest := sha256.Sum256(archive)
	signer := rigUpdateKey()
	if tamper {
		other := sha256.Sum256([]byte("someone else"))
		signer = ed25519.NewKeyFromSeed(other[:])
	}
	manifest, err := json.Marshal(map[string]any{
		"version":   "2.0.1",
		"notes":     "Rig update",
		"date":      time.Now().UTC().Format(time.RFC3339),
		"url":       "http://127.0.0.1:" + updatePort() + rigUpdateArchive,
		"size":      len(archive),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(signer, digest[:])),
	})
	must(t, err)
	return &updateFeed{manifest: manifest, archive: archive, served: map[string]int{}}
}

func tarGzipBinary(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	must(t, tw.WriteHeader(&tar.Header{Name: rigUpdateFile, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}))
	_, err := tw.Write(binary)
	must(t, err)
	must(t, tw.Close())
	must(t, gz.Close())
	return buf.Bytes()
}

func (f *updateFeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.served[r.URL.Path]++
	f.mu.Unlock()
	switch r.URL.Path {
	case rigUpdateManifest:
		w.Write(f.manifest)
	case rigUpdateArchive:
		w.Write(f.archive)
	default:
		http.NotFound(w, r)
	}
}

func (f *updateFeed) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.served[path]
}

// serveUpdateFeed listens on the port the builds were linked with and serves feed until the test ends.
func serveUpdateFeed(t *testing.T, feed *updateFeed) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:"+updatePort())
	must(t, err)
	server := &http.Server{Handler: feed, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
}

func updatePort() string {
	if port := os.Getenv(rigUpdatePortEnv); port != "" {
		return port
	}
	return rigUpdateDefault
}

// installedApp copies the old build into a directory of its own, so the update replaces that copy and never the
// build the other journey uses, and points the launch at it.
func installedApp(t *testing.T) string {
	t.Helper()
	oldBuild := os.Getenv("NVM_RIG_UPDATE_OLD_BIN")
	if oldBuild == "" {
		t.Skip("the update journeys need scripts/rig/update-journey.sh")
	}
	data, err := os.ReadFile(oldBuild)
	must(t, err)
	target := filepath.Join(t.TempDir(), rigUpdateFile)
	must(t, os.WriteFile(target, data, 0o755))
	t.Setenv("NVM_RIG_BIN", target)
	return target
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

// TestJourneyUpdateFoundDownloadedAndOffersARestart is J24 on the rig: the app finds a newer signed release on the
// feed, downloads and installs it in the background, and the palette offers the restart.
func TestJourneyUpdateFoundDownloadedAndOffersARestart(t *testing.T) {
	newBuild := os.Getenv("NVM_RIG_UPDATE_BIN_NEW")
	if newBuild == "" {
		t.Skip("the update journeys need scripts/rig/update-journey.sh")
	}
	installed := installedApp(t)
	feed := newUpdateFeed(t, newBuild, false)
	serveUpdateFeed(t, feed)
	before := fileDigest(t, installed)

	app := Launch(t, Options{Home: os.Getenv("NVM_RIG_HOME")})
	must(t, app.SummonPalette())
	_, err := app.WaitFor("is ready. Restart to finish updating", rigUpdateWait)
	must(t, err)
	shoot(t, app, "update-1-ready-prompt")
	if feed.count(rigUpdateArchive) != 1 {
		t.Fatalf("the archive was served %d times, want once", feed.count(rigUpdateArchive))
	}
	if fileDigest(t, installed) == before || fileDigest(t, installed) != fileDigest(t, newBuild) {
		t.Fatal("the installed app is not the signed update")
	}
	must(t, app.DismissWithEscape())
}

// TestJourneyUpdateRefusesAnArchiveThatIsNotSigned is the bad-signature case of J24: the app refuses the archive,
// says so in the palette, and keeps the installed app unchanged.
func TestJourneyUpdateRefusesAnArchiveThatIsNotSigned(t *testing.T) {
	newBuild := os.Getenv("NVM_RIG_UPDATE_BIN_NEW")
	if newBuild == "" {
		t.Skip("the update journeys need scripts/rig/update-journey.sh")
	}
	installed := installedApp(t)
	serveUpdateFeed(t, newUpdateFeed(t, newBuild, true))
	before := fileDigest(t, installed)

	app := Launch(t, Options{Home: os.Getenv("NVM_RIG_HOME")})
	must(t, app.SummonPalette())
	_, err := app.WaitFor("Update install failed", rigUpdateWait)
	must(t, err)
	shoot(t, app, "update-2-refused")
	if fileDigest(t, installed) != before {
		t.Fatal("an unsigned archive changed the installed app")
	}
	must(t, app.DismissWithEscape())
}
