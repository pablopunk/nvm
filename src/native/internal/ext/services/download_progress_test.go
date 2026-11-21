package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

type progressSurface struct {
	mu    sync.Mutex
	shows []api.IndicatorInput
	hides []string
}

func (s *progressSurface) ShowIndicator(owner string, input api.IndicatorInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shows = append(s.shows, input)
}

func (s *progressSurface) HideIndicator(owner, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hides = append(s.hides, owner+"/"+id)
}

// slowFiles reads the body in fixed chunks, letting the fake clock run between them, then publishes it.
type slowFiles struct {
	downloadTestFiles
	clock *fake.Clock
	chunk int
	tick  time.Duration
}

func (f slowFiles) WriteStream(ctx context.Context, path string, source io.Reader, max int64, overwrite bool, admit func() error) (platform.StreamFileResult, error) {
	var data bytes.Buffer
	buffer := make([]byte, f.chunk)
	for {
		f.clock.Advance(f.tick)
		n, err := source.Read(buffer)
		data.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return platform.StreamFileResult{}, err
		}
	}
	return f.downloadTestFiles.WriteStream(ctx, path, &data, max, overwrite, admit)
}

func progressDownload(t *testing.T, body []byte, header http.Header, mutate func(*slowFiles), request api.DownloadRequest) (*progressSurface, error) {
	t.Helper()
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 200, Header: header, Body: body})
	clock := fake.NewClock()
	files := slowFiles{downloadTestFiles: downloadTestFiles{FS: real.NewPorts().FS}, clock: clock, chunk: 1000, tick: 100 * time.Millisecond}
	if mutate != nil {
		mutate(&files)
	}
	surface := &progressSurface{}
	h := desktopHandler(t, Deps{Files: files, Network: port, Indicators: surface, Clock: clock}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	request.URL, request.Path, request.IndicatorID = "https://example.com/big", filepath.Join(t.TempDir(), "big.bin"), "dl"
	_, err := h.download(context.Background(), request)
	return surface, err
}

func TestDownloadProgressIsThrottledAndReportsBytesOfTotal(t *testing.T) {
	surface, err := progressDownload(t, bytes.Repeat([]byte{1}, 10000), nil, nil, api.DownloadRequest{IndicatorTitle: "Fetching"})
	if err != nil {
		t.Fatal(err)
	}
	// One initial update, then one per 250 ms of the ten 100 ms reads, not one per read.
	if len(surface.shows) != 4 {
		t.Fatalf("updates = %+v; want 4 throttled updates", surface.shows)
	}
	first, last := surface.shows[0], surface.shows[len(surface.shows)-1]
	if first.Subtitle != "0 B of 9.8 KB" || first.Total != 10000 || first.Title != "Fetching" || first.Status != api.IndicatorLoading {
		t.Fatalf("first update = %+v", first)
	}
	if last.Value != 9000 || last.Subtitle != "8.8 KB of 9.8 KB" {
		t.Fatalf("last update = %+v", last)
	}
	if len(surface.hides) != 0 {
		t.Fatalf("a finished download hid the indicator the extension replaces: %v", surface.hides)
	}
}

func TestDownloadProgressWithoutALengthShowsBytesOnly(t *testing.T) {
	surface := &progressSurface{}
	clock := fake.NewClock()
	p := &downloadProgress{ctx: context.Background(), surface: surface, clock: clock, owner: "x", id: "dl", title: "t", total: -1}
	p.emit()
	clock.Advance(time.Second)
	p.advance(2 << 20)
	if len(surface.shows) != 2 || surface.shows[1].Subtitle != "2.0 MB" || surface.shows[1].Total != 0 || surface.shows[1].Value != 0 {
		t.Fatalf("unknown length updates = %+v", surface.shows)
	}
}

func TestDownloadProgressHidesTheIndicatorOnFailure(t *testing.T) {
	surface, err := progressDownload(t, bytes.Repeat([]byte{1}, 4000), nil, nil, api.DownloadRequest{MediaTypes: []string{"image/png"}})
	if err == nil {
		t.Fatal("a mismatched media type succeeded")
	}
	if len(surface.hides) != 1 || surface.hides[0] != "acme.desktop/dl" {
		t.Fatalf("hides after failure = %v", surface.hides)
	}
}

func TestDownloadProgressStopsAndHidesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 200, Body: bytes.Repeat([]byte{1}, 10000)})
	clock := fake.NewClock()
	files := slowFiles{downloadTestFiles: downloadTestFiles{FS: real.NewPorts().FS, beforeAdmit: cancel}, clock: clock, chunk: 1000, tick: 100 * time.Millisecond}
	surface := &progressSurface{}
	h := desktopHandler(t, Deps{Files: files, Network: port, Indicators: surface, Clock: clock}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	_, err := h.download(ctx, api.DownloadRequest{URL: "https://example.com", Path: filepath.Join(t.TempDir(), "out"), IndicatorID: "dl"})
	if err == nil {
		t.Fatal("cancelled download succeeded")
	}
	if len(surface.hides) != 1 {
		t.Fatalf("hides after cancellation = %v", surface.hides)
	}
	shown := len(surface.shows)
	p := &downloadProgress{ctx: ctx, surface: surface, clock: clock, owner: "x", id: "dl"}
	p.emit()
	if len(surface.shows) != shown {
		t.Fatal("progress was reported after cancellation")
	}
}

func TestDownloadWithoutAnIndicatorSurfaceStillSucceeds(t *testing.T) {
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 200, Body: []byte("body")})
	files := downloadTestFiles{FS: real.NewPorts().FS}
	h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	if _, err := h.download(context.Background(), api.DownloadRequest{URL: "https://example.com", Path: filepath.Join(t.TempDir(), "out"), IndicatorID: "dl"}); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadWithoutStreamingFilesIsUnavailable(t *testing.T) {
	port := fake.NewHTTP()
	h := desktopHandler(t, Deps{Files: struct{ platform.FS }{real.NewPorts().FS}, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	_, err := h.download(context.Background(), api.DownloadRequest{URL: "https://example.com", Path: filepath.Join(t.TempDir(), "out")})
	if err == nil || !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("download without streaming files = %v; want unavailable", err)
	}
}
