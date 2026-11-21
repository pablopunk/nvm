package manager

import (
	"context"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/picture"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// PictureSource is a host-only read grant. Current must be checked again before publishing pixels or a denial.
// The view owns Source and closes it on success, cancellation and failure.
type PictureSource struct {
	Path    string
	Source  platform.ReadSource
	Current func() bool
}

// OpenPictureSource binds deferred file access to the installation that created the view.
func (m *Manager) OpenPictureSource(ctx context.Context, id, path string, identity any) (PictureSource, error) {
	host := m.pictureHost(id, identity)
	if host == nil {
		return PictureSource{}, expiredPictureAuthority()
	}
	source, err := host.OpenPictureSource(ctx, path)
	if err != nil {
		return PictureSource{}, err
	}
	current := func() bool { return host.PictureAccessCurrent() && m.pictureHost(id, identity) == host }
	if !current() {
		_ = source.Close()
		return PictureSource{}, expiredPictureAuthority()
	}
	return PictureSource{Source: source, Current: current}, nil
}

func (m *Manager) pictureHost(id string, identity any) *services.Handler {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || !m.isRunnable(e) || e.host == nil {
		return nil
	}
	revision, _ := identity.(*byte)
	if !e.builtin && (e.runtime == nil || revision == nil || revision != e.snapshotIdentity) {
		return nil
	}
	for _, capability := range e.bundle.Manifest.Capabilities {
		if capability == api.CapabilityDesktopFiles {
			return e.host
		}
	}
	return nil
}

func expiredPictureAuthority() error {
	return fmt.Errorf("%w: the picture's extension permission has changed; reopen its view", api.ErrPermission)
}

// OpenGridPictureSource validates host provenance before opening the installation's source.
func (m *Manager) OpenGridPictureSource(ctx context.Context, scope *picture.Scope, binding picture.Binding, ref string) (PictureSource, error) {
	authorizedPath, ok := scope.Path(binding, ref)
	if !ok {
		return PictureSource{}, fmt.Errorf("%w: picture scope expired or reference was not issued by this invocation", api.ErrInvalidInput)
	}
	host := m.pictureHost(binding.ExtensionID, binding.Identity)
	if host == nil {
		return PictureSource{}, expiredPictureAuthority()
	}
	path, source, err := host.OpenGridPictureSource(ctx, ref)
	if path != "" && path != authorizedPath {
		if source != nil {
			_ = source.Close()
		}
		return PictureSource{}, fmt.Errorf("%w: picture source changed its provenance", api.ErrInvalidInput)
	}
	current := func() bool {
		return host.PictureAccessCurrent() && scope.Current() && m.pictureHost(binding.ExtensionID, binding.Identity) == host
	}
	if !current() {
		if source != nil {
			_ = source.Close()
		}
		return PictureSource{}, expiredPictureAuthority()
	}
	if err != nil {
		return PictureSource{Path: path, Current: current}, err
	}
	return PictureSource{Path: path, Source: source, Current: current}, nil
}
