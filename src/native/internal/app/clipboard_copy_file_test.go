package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type imageAuthority struct{ allow bool }

func (a *imageAuthority) MayCopyImage(_ string, _ any, _ bool) bool { return a.allow }

func TestCopyImageFileWritesPixelsAndRechecksAuthority(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "revoked before completion"}[revoked], func(t *testing.T) {
			fs := fake.NewFS()
			if err := fs.MkdirAll("/Desktop", 0700); err != nil {
				t.Fatal(err)
			}
			img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
			img.Set(1, 0, color.NRGBA{R: 255, A: 255})
			var source bytes.Buffer
			if err := png.Encode(&source, img); err != nil {
				t.Fatal(err)
			}
			if err := fs.WriteFile("/Desktop/shot.png", source.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			clipboard := fake.NewClipboard()
			queue := make(completionUIQueue, 1)
			p := &App{ports: platform.Ports{FS: fs, Clipboard: clipboard}, uiQueue: queue}
			host := &recordingHost{}
			authority := &imageAuthority{allow: true}
			p.copyImageFile(host, authority, "local.screenshots", "/Desktop/shot.png", new(byte))
			complete := <-queue
			authority.allow = !revoked
			complete()
			got, err := clipboard.ReadImagePNG()
			if revoked {
				if err == nil || host.hidden || len(host.toasts) != 1 {
					t.Fatalf("revoked: image %d, hidden %v, toasts %v", len(got), host.hidden, host.toasts)
				}
				return
			}
			if err != nil || !host.hidden {
				t.Fatalf("copy: %v hidden %v", err, host.hidden)
			}
			decoded, err := png.Decode(bytes.NewReader(got))
			if err != nil || decoded.Bounds().Dx() != 2 || color.NRGBAModel.Convert(decoded.At(1, 0)) != (color.NRGBA{R: 255, A: 255}) {
				t.Fatalf("pixels: %v %v", decoded, err)
			}
		})
	}
}

func TestCopyImageFileDenialDoesNotReadOrCopy(t *testing.T) {
	clipboard := fake.NewClipboard()
	p := &App{ports: platform.Ports{FS: fake.NewFS(), Clipboard: clipboard}}
	host := &recordingHost{}
	p.copyImageFile(host, &imageAuthority{}, "x", "/private/image.png", nil)
	if _, err := clipboard.ReadImagePNG(); err == nil || len(host.toasts) != 1 || host.hidden {
		t.Fatalf("denial: %v %+v", err, host)
	}
}

type obsoleteCopyHost struct{ recordingHost }

func (*obsoleteCopyHost) BeginAction() func() bool { return func() bool { return false } }

func TestCopyImageDoesNotCompleteAfterNavigation(t *testing.T) {
	fs := fake.NewFS()
	if err := fs.MkdirAll("/Desktop", 0700); err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile("/Desktop/shot.png", source.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	clipboard := fake.NewClipboard()
	queue := make(completionUIQueue, 1)
	p := &App{ports: platform.Ports{FS: fs, Clipboard: clipboard}, uiQueue: queue}
	host := &obsoleteCopyHost{}
	p.copyImageFile(host, &imageAuthority{allow: true}, "local.screenshots", "/Desktop/shot.png", new(byte))
	(<-queue)()
	if _, err := clipboard.ReadImagePNG(); err == nil || host.hidden || len(host.toasts) != 0 {
		t.Fatalf("obsolete result: %v %+v", err, host)
	}
}
