package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// downloadProgressInterval bounds how often a download redraws its indicator, whatever the read size.
const downloadProgressInterval = 250 * time.Millisecond

// downloadProgress turns bytes read into throttled updates of the extension's own indicator. It runs on the reading
// goroutine, so no update can arrive after the download call returns, and it stops reporting once the call is cancelled.
type downloadProgress struct {
	ctx      context.Context
	surface  IndicatorSurface
	clock    platform.Clock
	owner    string
	id       string
	title    string
	total    int64
	received int64
	last     time.Time
	shown    bool
}

// newDownloadProgress returns nil when the request asked for no indicator or this host draws none: progress is
// decoration and never fails a download.
func (h *Handler) newDownloadProgress(ctx context.Context, request api.DownloadRequest, total int64) *downloadProgress {
	if request.IndicatorID == "" || h.deps.Indicators == nil {
		return nil
	}
	title := request.IndicatorTitle
	if title == "" {
		title = "Downloading…"
	}
	p := &downloadProgress{ctx: ctx, surface: h.deps.Indicators, clock: h.deps.Clock, owner: h.manifest.ID, id: request.IndicatorID, title: title, total: total}
	p.emit()
	return p
}

func (p *downloadProgress) wrap(source io.Reader) io.Reader {
	if p == nil {
		return source
	}
	return &progressReader{source: source, progress: p}
}

func (p *downloadProgress) advance(n int) {
	p.received += int64(n)
	if p.clock.Now().Sub(p.last) >= downloadProgressInterval {
		p.emit()
	}
}

func (p *downloadProgress) emit() {
	if p.ctx.Err() != nil {
		return
	}
	p.last = p.clock.Now()
	p.shown = true
	input := api.IndicatorInput{ID: p.id, Title: p.title, Status: api.IndicatorLoading, Subtitle: byteProgressLabel(p.received, p.total)}
	if p.total > 0 {
		input.Value, input.Total = int(min(p.received, p.total)), int(p.total)
	}
	p.surface.ShowIndicator(p.owner, input)
}

// abandon removes the progress indicator after a failure or cancellation, so none is left claiming work in flight.
// Success leaves it for the extension, which replaces it with its own result.
func (p *downloadProgress) abandon() {
	if p != nil && p.shown {
		p.surface.HideIndicator(p.owner, p.id)
	}
}

type progressReader struct {
	source   io.Reader
	progress *downloadProgress
}

func (r *progressReader) Read(buffer []byte) (int, error) {
	n, err := r.source.Read(buffer)
	if n > 0 {
		r.progress.advance(n)
	}
	return n, err
}

// byteProgressLabel reads "3.2 MB of 8.0 MB", or just the bytes received when the server sent no length.
func byteProgressLabel(received, total int64) string {
	if total > 0 {
		return humanBytes(received) + " of " + humanBytes(total)
	}
	return humanBytes(received)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
