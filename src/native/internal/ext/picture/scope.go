// Package picture binds deferred thumbnail reads to host-observed invocation evidence and a live view.
package picture

import (
	"context"
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"net/url"
	"path/filepath"
	"reflect"
)

// Binding never crosses extension IPC. Session is minted by the host for one view.
type Binding struct {
	ExtensionID string
	Identity    any
	Session     *byte
	Invocation  string
	Generation  uint64
	Version     uint64
}

// Scope owns a copy of exact URL/path provenance from one invocation or verified retained snapshot.
type Scope struct {
	ctx     context.Context
	binding Binding
	paths   map[[32]byte]string
}

func NewScope(ctx context.Context, binding Binding, evidence *api.SnapshotEvidence) *Scope {
	scope := &Scope{ctx: ctx, binding: binding, paths: make(map[[32]byte]string)}
	if ctx == nil || binding.Session == nil || evidence == nil || evidence.FilesAuthorityDenied || evidence.Overflow {
		return scope
	}
	for hash, paths := range evidence.ThumbnailPaths {
		if !evidence.ThumbnailHashes[hash] {
			continue
		}
		for _, path := range paths {
			if !filepath.IsAbs(path) {
				continue
			}
			denied := false
			for _, other := range evidence.DeniedPaths {
				if filepath.Clean(other) == filepath.Clean(path) {
					denied = true
					break
				}
			}
			if !denied {
				scope.paths[hash] = filepath.Clean(path)
			}
		}
	}
	return scope
}

func (s *Scope) Current() bool {
	return s != nil && s.ctx != nil && s.ctx.Err() == nil && s.binding.Session != nil
}

// Path accepts only an exact host-observed reference under its original invocation and view binding.
func (s *Scope) Path(binding Binding, ref string) (string, bool) {
	if !s.Current() || !sameBinding(s.binding, binding) {
		return "", false
	}
	path, ok := s.paths[sha256.Sum256([]byte(ref))]
	if !ok {
		return "", false
	}
	parsed, err := url.Parse(ref)
	if err != nil || parsed.Scheme != "file" || filepath.Clean(platform.FileURLPath(parsed)) != path {
		return "", false
	}
	return path, true
}

func sameBinding(a, b Binding) bool {
	identityEqual := a.Identity == nil && b.Identity == nil
	if a.Identity != nil && b.Identity != nil {
		kind := reflect.TypeOf(a.Identity)
		identityEqual = kind == reflect.TypeOf(b.Identity) && kind.Comparable() && a.Identity == b.Identity
	}
	return identityEqual && a.ExtensionID == b.ExtensionID && a.Session == b.Session && a.Invocation == b.Invocation && a.Generation == b.Generation && a.Version == b.Version
}

func (s *Scope) Matches(binding Binding) bool { return s.Current() && sameBinding(s.binding, binding) }
