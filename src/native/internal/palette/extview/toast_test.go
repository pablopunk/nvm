package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestToastIndicatorIsTitledWithTheActionAndTimedByTone(t *testing.T) {
	ok := toastIndicator("Keep awake", api.Toast{Message: "Mac will stay awake", Tone: "success"})
	if ok.Title != "Keep awake" || ok.Subtitle != "Mac will stay awake" || ok.Status != api.IndicatorSuccess || ok.DurationMs != 2200 {
		t.Fatalf("success toast = %+v", ok)
	}
	failed := toastIndicator("", api.Toast{Message: "Failed", Tone: "error"})
	if failed.Title != "Nevermind" || failed.Status != api.IndicatorError || failed.DurationMs != 4000 {
		t.Fatalf("error toast = %+v; want the default title, error status and four seconds", failed)
	}
}

func TestToastWithoutAnIndicatorSurfaceShowsUnderTheSearchField(t *testing.T) {
	s := &Session{deps: Deps{}}
	host := &fakeHost{}
	s.showToast(host, "acme.keep", "Keep", &api.Toast{Message: "Done"})
	if len(host.toasts) != 1 || host.toasts[0] != "Done" {
		t.Fatalf("toasts = %v; want the message under the search field", host.toasts)
	}
}
