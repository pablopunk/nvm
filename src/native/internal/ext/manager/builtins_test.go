package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// builtinSample answers search without a folder, a file or an enable.
type builtinSample struct{}

func (builtinSample) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.sample", Title: "Sample"}}
}

func (builtinSample) SearchItems(_ api.Context, query string) ([]api.Item, error) {
	return []api.Item{{ID: "hit", Title: "Built-in " + query}}, nil
}

func withBuiltIns(exts ...api.Extension) func(*Options) {
	return func(o *Options) { o.BuiltIns = exts }
}

func TestBuiltInAnswersSearchWithoutEnablingOrAFolder(t *testing.T) {
	h := newHarness(t, withBuiltIns(builtinSample{}))
	result := resultFor(h.search("two"), "nevermind.sample")
	if result == nil || result.Failure != nil || len(result.Items) != 1 || result.Items[0].Title != "Built-in two" {
		t.Fatalf("built-in result = %+v", result)
	}
}

func TestBuiltInIsNotListedAsAFolderExtension(t *testing.T) {
	h := newHarness(t, withBuiltIns(builtinSample{}))
	for _, listing := range h.m.List() {
		if listing.ID == "nevermind.sample" {
			t.Fatalf("built-in listed among folder extensions: %+v", listing)
		}
	}
	if !h.m.IsBuiltIn("nevermind.sample") {
		t.Errorf("IsBuiltIn(nevermind.sample) = false")
	}
}

func TestBuiltInCannotBeDisabledUninstalledOrRead(t *testing.T) {
	h := newHarness(t, withBuiltIns(builtinSample{}))
	if err := h.m.Disable("nevermind.sample"); !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Disable = %v, want ErrBuiltIn", err)
	}
	if err := h.m.Enable("nevermind.sample"); !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Enable = %v, want ErrBuiltIn", err)
	}
	if err := h.m.Uninstall("nevermind.sample"); !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Uninstall = %v, want ErrBuiltIn", err)
	}
	if _, err := h.m.Source("nevermind.sample"); !errors.Is(err, ErrBuiltIn) {
		t.Errorf("Source = %v, want ErrBuiltIn", err)
	}
}

func TestRescanKeepsBuiltInsRunning(t *testing.T) {
	h := newHarness(t, withBuiltIns(builtinSample{}))
	if err := h.m.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	if result := resultFor(h.m.Search(ctx, "x"), "nevermind.sample"); result == nil {
		t.Fatalf("built-in gone after a rescan")
	}
}
