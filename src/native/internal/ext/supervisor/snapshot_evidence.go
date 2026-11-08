package supervisor

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"path/filepath"
)

// observeSnapshotService records only authority exercised by the host files service.
func observeSnapshotService(e *api.SnapshotEvidence, call protocol.ServiceCall, reply protocol.ServiceReply) *api.SnapshotDenial {
	if call.Service != protocol.ServiceDesktopFiles {
		return nil
	}
	if reply.SnapshotPermissionFailure != nil {
		var args protocol.PathArgs
		if call.Op != protocol.OpThumbnail || json.Unmarshal(call.Args, &args) != nil || !filepath.IsAbs(args.Path) || len(args.Path) > 4096 {
			e.FilesAuthorityDenied = true
			return &api.SnapshotDenial{Failure: reply.SnapshotPermissionFailure, AuthorityDenied: true}
		}
		path := filepath.Clean(args.Path)
		for _, previous := range e.DeniedPaths {
			if previous == path {
				return &api.SnapshotDenial{Failure: reply.SnapshotPermissionFailure, Path: path}
			}
		}
		if len(e.DeniedPaths) >= 200 {
			e.FilesAuthorityDenied = true
			return &api.SnapshotDenial{Failure: reply.SnapshotPermissionFailure, AuthorityDenied: true}
		}
		e.DeniedPaths = append(e.DeniedPaths, path)
		return &api.SnapshotDenial{Failure: reply.SnapshotPermissionFailure, Path: path}
	}
	if reply.Failure != nil && reply.Failure.Code == api.CodePermission {
		switch call.Op {
		case protocol.OpThumbnail, protocol.OpFind, protocol.OpMetadata, protocol.OpFileURL:
			e.FilesAuthorityDenied = true
			return &api.SnapshotDenial{Failure: reply.Failure, AuthorityDenied: true}
		}
	}
	if call.Op == protocol.OpThumbnail && reply.Failure == nil {
		var args protocol.PathArgs
		if json.Unmarshal(call.Args, &args) != nil || !filepath.IsAbs(args.Path) || len(args.Path) > 4096 {
			return nil
		}
		var result protocol.ThumbnailResult
		if json.Unmarshal(reply.Data, &result) == nil && result.Found && result.URL != "" {
			hash := reply.PictureDigest
			if hash == ([32]byte{}) {
				hash = sha256.Sum256([]byte(result.URL))
			}
			path := filepath.Clean(args.Path)
			for _, previous := range e.ThumbnailPaths[hash] {
				if previous == path {
					return nil
				}
			}
			if e.ThumbnailSamples >= 200 {
				e.Overflow = true
				return nil
			}
			e.ThumbnailSamples++
			if e.ThumbnailHashes == nil {
				e.ThumbnailHashes = make(map[[32]byte]bool)
			}
			e.ThumbnailHashes[hash] = true
			if e.ThumbnailPaths == nil {
				e.ThumbnailPaths = make(map[[32]byte][]string)
			}
			e.ThumbnailPaths[hash] = append(e.ThumbnailPaths[hash], path)
		}
	}
	return nil
}
