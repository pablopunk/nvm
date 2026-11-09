package extview

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func TestDurableGridRequiresMatchingReadableImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	item := api.Item{ID: path, Title: "untrusted", Image: &api.Image{Src: durableGridReference(path)}}
	view := api.View{Items: []api.Item{item}}
	got, evidence, err := revalidateDurableGrid(context.Background(), real.NewPorts().FS, view)
	if err != nil || len(got.Items) != 1 || evidence.ThumbnailSamples != 1 {
		t.Fatalf("readable image = %+v, evidence = %+v, error = %v", got.Items, evidence, err)
	}
	view.Items[0].Image.Src = durableGridReference(filepath.Join(t.TempDir(), "other.png"))
	got, _, err = revalidateDurableGrid(context.Background(), real.NewPorts().FS, view)
	if err != nil || len(got.Items) != 0 {
		t.Fatalf("mismatched reference was accepted: %+v, %v", got.Items, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := revalidateDurableGrid(ctx, real.NewPorts().FS, view); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled validation: %v", err)
	}
}
