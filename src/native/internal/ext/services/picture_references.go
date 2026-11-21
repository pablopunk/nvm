package services

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/platform"
	"sync"
)

const maxPictureReferences = 2048

type pictureReferences struct {
	mu    sync.Mutex
	paths map[string]string
	order []string
}

func (p *pictureReferences) remember(ref, path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paths == nil {
		p.paths = make(map[string]string)
	}
	if _, ok := p.paths[ref]; ok {
		return
	}
	if len(p.order) == maxPictureReferences {
		delete(p.paths, p.order[0])
		p.order = p.order[1:]
	}
	p.paths[ref] = path
	p.order = append(p.order, ref)
}

// OpenGridPictureSource accepts only exact references returned by this installation's Thumbnail service.
func (h *Handler) OpenGridPictureSource(ctx context.Context, ref string) (string, platform.ReadSource, error) {
	h.pictureReferences.mu.Lock()
	path, ok := h.pictureReferences.paths[ref]
	h.pictureReferences.mu.Unlock()
	if !ok {
		return "", nil, invalid("this picture reference was not issued by the host")
	}
	source, err := h.OpenPictureSource(ctx, path)
	return path, source, err
}
