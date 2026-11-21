package clipboard

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fakeFetcher is a LinkFetcher whose behavior each test sets. It counts page fetches.
type fakeFetcher struct {
	html       func(url string) (Page, bool, error)
	image      func(url string) ([]byte, error)
	htmlCalls  atomic.Int32
	imageCalls chan string
}

// FetchHTML delegates to the test's page function and counts the call.
func (f *fakeFetcher) FetchHTML(url string) (Page, bool, error) {
	f.htmlCalls.Add(1)
	return f.html(url)
}

// FetchImage delegates to the test's image function and reports the address asked for.
func (f *fakeFetcher) FetchImage(url string) ([]byte, error) {
	if f.imageCalls != nil {
		f.imageCalls <- url
	}
	if f.image == nil {
		return nil, errors.New("no image")
	}
	return f.image(url)
}

// fakeImages records the pictures the service saves and names them by hash.
type fakeImages struct {
	mu    sync.Mutex
	saved map[string][]byte
}

// Save stores the picture under hash.png and returns that name.
func (i *fakeImages) Save(hash string, png []byte) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.saved == nil {
		i.saved = map[string][]byte{}
	}
	i.saved[hash] = png
	return ImageName(hash), nil
}

// fakeLinkStore keeps saved previews in memory, the way the history does.
type fakeLinkStore struct {
	mu       sync.Mutex
	previews map[string]LinkPreview
}

// PreviewFor returns the saved preview of url.
func (s *fakeLinkStore) PreviewFor(url string) (LinkPreview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.previews[url]
	return p, ok
}

// SetPreview saves preview under url.
func (s *fakeLinkStore) SetPreview(url string, preview LinkPreview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.previews == nil {
		s.previews = map[string]LinkPreview{}
	}
	s.previews[url] = preview
	return nil
}

// TestLinkServiceShowsLoadingLoadsOnceAndReportsSettlement: the placeholder is loading, a second Load while the
// first runs starts nothing, and the settled preview is kept.
func TestLinkServiceShowsLoadingLoadsOnceAndReportsSettlement(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{html: func(string) (Page, bool, error) {
		return Page{HTML: `<meta property="og:title" content="Hello">`, FinalURL: "https://a.dev/"}, true, nil
	}}
	service := NewLinkService(LinkDeps{Fetcher: fetcher, Clock: fake.NewClock(), OnSettled: func(url string) { settled <- url }})
	if got := service.Peek("https://a.dev/").State; got != LinkLoading {
		t.Fatalf("placeholder state = %s", got)
	}
	service.Load("https://a.dev/")
	service.Load("https://a.dev/")
	if got := <-settled; got != "https://a.dev/" {
		t.Fatalf("settled %q", got)
	}
	if calls := fetcher.htmlCalls.Load(); calls != 1 {
		t.Fatalf("page fetched %d times, want once", calls)
	}
	if got := service.Peek("https://a.dev/").Title; got != "Hello" {
		t.Fatalf("title = %q", got)
	}
}

// TestLinkServiceMarksFailuresUnavailableWithoutRetry: a failed fetch is remembered and never retried in the
// session, as the 1.x cache does.
func TestLinkServiceMarksFailuresUnavailableWithoutRetry(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{html: func(string) (Page, bool, error) {
		return Page{}, false, errors.New("offline")
	}}
	service := NewLinkService(LinkDeps{Fetcher: fetcher, Clock: fake.NewClock(), OnSettled: func(url string) { settled <- url }})
	service.Load("https://a.dev/")
	<-settled
	service.Load("https://a.dev/")
	if got := service.Peek("https://a.dev/").State; got != LinkUnavailable {
		t.Fatalf("state = %s, want unavailable", got)
	}
	if calls := fetcher.htmlCalls.Load(); calls != 1 {
		t.Fatalf("page fetched %d times, want once", calls)
	}
}

// TestLinkServiceShowsMetadataThenImage: the first settlement has no image; the second carries the image name the
// host saved.
func TestLinkServiceShowsMetadataThenImage(t *testing.T) {
	images := make(chan string, 4)
	var service *LinkService
	fetcher := &fakeFetcher{
		html: func(string) (Page, bool, error) {
			return Page{HTML: `<meta property="og:title" content="Hi"><meta property="og:image" content="https://cdn.dev/a.png">`, FinalURL: "https://a.dev/"}, true, nil
		},
		image: func(url string) ([]byte, error) {
			if url != "https://cdn.dev/a.png" {
				t.Errorf("image fetched from %q", url)
			}
			return []byte("png-bytes"), nil
		},
	}
	saved := &fakeImages{}
	service = NewLinkService(LinkDeps{Fetcher: fetcher, Images: saved, Clock: fake.NewClock(), OnSettled: func(url string) { images <- service.Peek(url).ImageRef }})
	service.Load("https://a.dev/")
	if first := <-images; first != "" {
		t.Fatalf("first settlement carried an image: %q", first)
	}
	second := <-images
	if second == "" || second != ImageName(hashHex([]byte("png-bytes"))) {
		t.Fatalf("second settlement image = %q", second)
	}
}

// TestLinkServiceNeverExposesRemoteImageURL: when the host cannot fetch the image, the preview keeps its text and
// has no image name.
func TestLinkServiceNeverExposesRemoteImageURL(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{
		html: func(string) (Page, bool, error) {
			return Page{HTML: `<meta property="og:title" content="Hi"><meta property="og:image" content="http://127.0.0.1/x">`, FinalURL: "https://a.dev/"}, true, nil
		},
		image:      func(string) ([]byte, error) { return nil, errors.New("blocked") },
		imageCalls: make(chan string, 4),
	}
	service := NewLinkService(LinkDeps{Fetcher: fetcher, Images: &fakeImages{}, Clock: fake.NewClock(), OnSettled: func(url string) { settled <- url }})
	service.Load("https://a.dev/")
	<-settled
	if asked := <-fetcher.imageCalls; asked != "http://127.0.0.1/x" {
		t.Fatalf("image asked for %q", asked)
	}
	got := service.Peek("https://a.dev/")
	if got.ImageRef != "" || got.Title != "Hi" {
		t.Fatalf("status = %+v", got)
	}
}

// TestLinkServicePersistsReadyPreviewWithFetchedAt: a ready preview is saved with the history and the time it
// was fetched; an unavailable one is not saved.
func TestLinkServicePersistsReadyPreviewWithFetchedAt(t *testing.T) {
	clock := fake.NewClock()
	store := &fakeLinkStore{}
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{html: func(url string) (Page, bool, error) {
		if url == "https://bad.dev/" {
			return Page{}, false, errors.New("down")
		}
		return Page{HTML: `<meta property="og:title" content="Saved">`, FinalURL: url}, true, nil
	}}
	service := NewLinkService(LinkDeps{Fetcher: fetcher, Store: store, Clock: clock, OnSettled: func(url string) { settled <- url }})
	service.Load("https://ok.dev/")
	<-settled
	service.Load("https://bad.dev/")
	<-settled
	preview, ok := store.PreviewFor("https://ok.dev/")
	if !ok || preview.Title != "Saved" || !preview.FetchedAt.Equal(clock.Now()) {
		t.Fatalf("saved preview = %+v, ok %v", preview, ok)
	}
	if _, ok := store.PreviewFor("https://bad.dev/"); ok {
		t.Fatal("an unavailable preview was saved")
	}
}

// TestLinkServiceUsesSavedPreviewWithoutFetching: after a restart the saved preview is shown and nothing is
// fetched.
func TestLinkServiceUsesSavedPreviewWithoutFetching(t *testing.T) {
	store := &fakeLinkStore{previews: map[string]LinkPreview{
		"https://saved.dev/": {URL: "https://saved.dev/", Title: "Kept", ImageRef: ImageName(hashHex([]byte("x")))},
	}}
	fetcher := &fakeFetcher{html: func(string) (Page, bool, error) {
		t.Fatal("a saved preview was fetched again")
		return Page{}, false, nil
	}}
	service := NewLinkService(LinkDeps{Fetcher: fetcher, Store: store, Clock: fake.NewClock()})
	service.Load("https://saved.dev/")
	got := service.Peek("https://saved.dev/")
	if got.State != LinkReady || got.Title != "Kept" || got.ImageRef == "" {
		t.Fatalf("status = %+v", got)
	}
	if fetcher.htmlCalls.Load() != 0 {
		t.Fatal("page fetched")
	}
}
