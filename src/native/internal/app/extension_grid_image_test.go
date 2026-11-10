package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/picture"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type gridSourceStub struct {
	open func(context.Context, *picture.Scope, picture.Binding, string) (manager.PictureSource, error)
}

func (s gridSourceStub) OpenGridPictureSource(ctx context.Context, scope *picture.Scope, binding picture.Binding, ref string) (manager.PictureSource, error) {
	return s.open(ctx, scope, binding, ref)
}

func TestScopedGridOpenDenialsAndPublicationRevocation(t *testing.T) {
	for _, kind := range []string{"path", "authority", "ordinary", "revoked", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queue := make(completionUIQueue, 1)
			p := &App{uiQueue: queue}
			var current atomic.Bool
			current.Store(true)
			identity := new(byte)
			request := extview.GridImageRequest{ExtensionID: "files", Identity: identity, Source: "issued-reference"}
			source := gridSourceStub{open: func(_ context.Context, _ *picture.Scope, binding picture.Binding, ref string) (manager.PictureSource, error) {
				if binding.ExtensionID != request.ExtensionID || ref != request.Source || binding.Identity != identity {
					panic("lost source scope")
				}
				grant := manager.PictureSource{Path: "/private/image.png", Current: current.Load}
				switch kind {
				case "path":
					return grant, fs.ErrPermission
				case "authority":
					return manager.PictureSource{}, api.ErrPermission
				default:
					return grant, errors.New("not an image")
				}
			}}
			var result *extview.GridImageResult
			p.loadScopedGridImage(ctx, source, request, func(got extview.GridImageResult) { result = &got })
			var publish func()
			select {
			case publish = <-queue:
			case <-time.After(5 * time.Second):
				t.Fatal("image completion was not dispatched")
			}
			if result != nil {
				t.Fatal("worker published before UI dispatch")
			}
			if kind == "revoked" {
				current.Store(false)
			}
			if kind == "canceled" {
				cancel()
			}
			publish()
			if kind == "canceled" {
				if result != nil {
					t.Fatal("canceled image published")
				}
				return
			}
			if result == nil {
				t.Fatal("completion missing")
			}
			if kind == "ordinary" {
				if result.Denial != nil {
					t.Fatal("decode or ordinary load failure became a permission denial")
				}
				return
			}
			if result.Denial == nil {
				t.Fatal("permission failure was lost")
			}
			if kind == "path" {
				if result.Denial.Path != "/private/image.png" || result.Denial.AuthorityDenied {
					t.Fatal("path denial lost its scope")
				}
			} else if !result.Denial.AuthorityDenied {
				t.Fatal("revocation was not authority-wide")
			}
		})
	}
}

type observedGridSource struct {
	platform.ReadSource
	denied bool
	closed chan struct{}
}

func (s *observedGridSource) Read(data []byte) (int, error) {
	if s.denied {
		return 0, errors.Join(fs.ErrPermission, platform.ErrSourceReadDenied)
	}
	return s.ReadSource.Read(data)
}
func (s *observedGridSource) Close() error { close(s.closed); return s.ReadSource.Close() }

func TestScopedGridSourceRendersOrReportsReadDenialAndCloses(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "render", true: "read-denied"}[denied], func(t *testing.T) {
			files := fake.NewFS()
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
				t.Fatal(err)
			}
			files.Seed("/image.png", encoded.Bytes())
			opened, err := files.OpenReadNoFollow("/image.png")
			if err != nil {
				t.Fatal(err)
			}
			source := &observedGridSource{ReadSource: opened, denied: denied, closed: make(chan struct{})}
			grants := gridSourceStub{open: func(context.Context, *picture.Scope, picture.Binding, string) (manager.PictureSource, error) {
				return manager.PictureSource{Path: "/image.png", Source: source, Current: func() bool { return true }}, nil
			}}
			queue := make(completionUIQueue, 1)
			p := &App{uiQueue: queue, files: newTestFiles(t, "/home", nil)}
			var result extview.GridImageResult
			p.loadScopedGridImage(context.Background(), grants, extview.GridImageRequest{}, func(got extview.GridImageResult) { result = got })
			select {
			case publish := <-queue:
				publish()
			case <-time.After(5 * time.Second):
				t.Fatal("read completion not dispatched")
			}
			select {
			case <-source.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("read handle was not closed")
			}
			if denied {
				if result.Bitmap != nil || result.Denial == nil || result.Denial.Path != "/image.png" || result.Denial.AuthorityDenied {
					t.Fatal("source read permission error lost path scope")
				}
			} else if result.Bitmap == nil || result.Denial != nil {
				t.Fatal("authorized source did not render")
			}
		})
	}
}
