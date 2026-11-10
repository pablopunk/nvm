package app

import (
	"bytes"
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"image"
	"image/png"
	"io/fs"
	"testing"
)

func TestGridPictureReopensSourceBeforeMemoryAndDiskHits(t *testing.T) {
	for _, disk := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "disk"}[disk], func(t *testing.T) {
			service := newTestFiles(t, "/home", nil)
			files := service.fsys.(*fake.FS)
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
				t.Fatal(err)
			}
			files.Seed("/image.png", encoded.Bytes())
			service.gridPictures = newAsyncCache[*ui.Bitmap](service.ctx, 0, 1, gridPictureEntries, nil)
			if service.gridPicture(context.Background(), "/image.png") == nil {
				t.Fatal("could not prime grid picture")
			}
			if disk {
				service.gridPictures = newAsyncCache[*ui.Bitmap](service.ctx, 0, 1, gridPictureEntries, nil)
			}
			files.FailPath(fake.OpOpenReadNoFollow, "/image.png", fs.ErrPermission)
			if service.gridPicture(context.Background(), "/image.png") != nil {
				t.Fatal("cache bypassed source read denial")
			}
			files.FailPath(fake.OpOpenReadNoFollow, "/image.png", nil)
			if service.gridPicture(context.Background(), "/image.png") == nil {
				t.Fatal("fresh authorization did not restore the picture")
			}
		})
	}
}
