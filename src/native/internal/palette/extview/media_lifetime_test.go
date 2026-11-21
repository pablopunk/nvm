package extview

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestImageRequestDeduplicatesAndCancelsOnClose(t *testing.T) {
	var calls int
	var requestContext context.Context
	var complete func(*ui.Bitmap)
	s := NewSession(Deps{LoadImage: func(ctx context.Context, ref string, done func(*ui.Bitmap)) {
		calls++
		requestContext = ctx
		complete = done
	}}, "test", api.View{})
	image := &api.Image{Src: "https://example.com/image.png"}
	s.bitmap(image)
	s.bitmap(image)
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
	s.close()
	if requestContext.Err() != context.Canceled {
		t.Fatal("view close did not cancel image")
	}
	complete(nil)
	if len(s.images) != 0 {
		t.Fatal("dismissed view retained a late image")
	}
}
