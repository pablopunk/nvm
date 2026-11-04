package update

import "context"

// startDownloadLocked starts the download of offer in the background and returns the state that shows it. The
// download is cancelled by a channel switch, and its result is dropped when the channel changed while it ran (epoch).
// The caller holds s.mu.
func (s *Service) startDownloadLocked(ctx context.Context, epoch int, offer Offer) State {
	downloadCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.runDownload(downloadCtx, epoch, offer)
	return State{Status: StatusDownloading, CurrentVersion: s.state.CurrentVersion, LatestVersion: offer.Version}
}

func (s *Service) cancelDownloadLocked() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Service) runDownload(ctx context.Context, epoch int, offer Offer) {
	err := s.engine.Install(ctx, func(done, total int64) {
		s.reportProgress(epoch, offer, done, total)
	})
	s.finishDownload(ctx, epoch, offer, err)
}

// reportProgress moves the download fraction. Changes under a whole percent are not announced, so a fast download
// does not flood the menu and the palette.
func (s *Service) reportProgress(epoch int, offer Offer, done, total int64) {
	if total <= 0 {
		return
	}
	fraction := float64(done) / float64(total)
	s.mu.Lock()
	if epoch != s.epoch || s.state.Status != StatusDownloading {
		s.mu.Unlock()
		return
	}
	if percent(fraction) == percent(s.state.Progress) {
		s.mu.Unlock()
		return
	}
	s.state = State{Status: StatusDownloading, CurrentVersion: s.state.CurrentVersion, LatestVersion: offer.Version, Progress: fraction}
	state := s.state
	s.mu.Unlock()
	s.notify(state)
}

// finishDownload records that the update is installed and waits for its restart, or that the install failed. A
// cancelled download, one of an older channel, is dropped.
func (s *Service) finishDownload(ctx context.Context, epoch int, offer Offer, err error) {
	s.mu.Lock()
	if epoch != s.epoch || ctx.Err() != nil && err != nil {
		s.mu.Unlock()
		return
	}
	s.cancel = nil
	state := State{Status: StatusDownloaded, CurrentVersion: s.state.CurrentVersion, LatestVersion: offer.Version}
	if err != nil {
		state = State{Status: StatusFailed, CurrentVersion: s.state.CurrentVersion, LatestVersion: offer.Version, Stage: StageInstall, Message: err.Error()}
	}
	s.state = state
	s.mu.Unlock()
	s.notify(state)
}
