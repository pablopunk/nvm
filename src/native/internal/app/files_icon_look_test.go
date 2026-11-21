package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

type changingIconLooks struct {
	look    string
	changed bool
}

func (c *changingIconLooks) FilePicture(context.Context, string, int) ([]byte, error) {
	return nil, nil
}

func (c *changingIconLooks) IconLook(context.Context) (string, bool) { return c.look, c.changed }

func (c *changingIconLooks) ThemedIcon(context.Context, string, int) ([]byte, error) {
	return []byte("themed"), nil
}

func TestAChangedIconLookForgetsTheDrawnIcons(t *testing.T) {
	redraws := 0
	looks := &changingIconLooks{look: "launch"}
	service := &fileService{
		icons:    newAsyncCache[*ui.Bitmap](context.Background(), 0, 1, 4, nil),
		pictures: looks,
		redraw:   func() { redraws++ },
		ctx:      context.Background(),
	}
	service.icons.Put("app", nil)

	service.applyIconLook(context.Background(), looks)
	if service.icons.Len() != 1 || redraws != 0 {
		t.Fatalf("the look at launch cleared the icons: len %d, redraws %d", service.icons.Len(), redraws)
	}

	looks.look, looks.changed = "tinted", true
	service.applyIconLook(context.Background(), looks)
	if service.icons.Len() != 0 || redraws != 1 {
		t.Fatalf("a new look kept the icons: len %d, redraws %d", service.icons.Len(), redraws)
	}
	data, err := service.systemIconPNG("/Applications/Safari.app")
	if err != nil || string(data) != "themed" {
		t.Fatalf("icon in a new look = %q, %v", data, err)
	}
}
