package app

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/picture"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io/fs"
)

type gridPictureSources interface {
	OpenGridPictureSource(context.Context, *picture.Scope, picture.Binding, string) (manager.PictureSource, error)
}

func (p *App) loadScopedGridImage(ctx context.Context, m gridPictureSources, request extview.GridImageRequest, done func(extview.GridImageResult)) {
	go func() {
		select {
		case gridPicturePermits <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-gridPicturePermits }()
		grant, err := m.OpenGridPictureSource(ctx, request.Scope, request.Binding(), request.Source)
		result := extview.GridImageResult{Authorized: grant.Current}
		switch {
		case errors.Is(err, api.ErrPermission):
			result.Denial = &api.SnapshotDenial{AuthorityDenied: true, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was revoked."}}
		case errors.Is(err, fs.ErrPermission) && grant.Path != "":
			if p.files != nil {
				p.files.forgetGridPicture(grant.Path)
			}
			result.Denial = &api.SnapshotDenial{Path: grant.Path, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was denied."}}
		case err == nil:
			var renderErr error
			defer grant.Source.Close()
			if p.files != nil {
				result.Bitmap, renderErr = p.files.gridPictureResult(ctx, grant.Path, grant.Source)
			} else {
				data, err := thumbs.RenderSource(grant.Source, thumbs.GridEdge)
				renderErr = err
				if err == nil {
					result.Bitmap, _ = ui.DecodeBitmap(data)
				}
			}
			if errors.Is(renderErr, platform.ErrSourceReadDenied) {
				if p.files != nil {
					p.files.forgetGridPicture(grant.Path)
				}
				result.Denial = &api.SnapshotDenial{Path: grant.Path, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was denied."}}
			}

		}
		if ctx.Err() != nil {
			return
		}
		p.onUI(func() {
			if ctx.Err() != nil {
				return
			}
			if result.Authorized != nil && !result.Authorized() {
				result.Bitmap = nil
				result.Denial = &api.SnapshotDenial{AuthorityDenied: true, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was revoked."}}
			}
			done(result)
		})
	}()
}
