package extview

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Defaults of a data source's snapshot windows, as the Electron host uses them: a snapshot younger than the fresh
// window is shown without a load, and an older one is shown at once and loaded again in place.
const (
	defaultFreshMs = 60_000
	defaultStaleMs = 300_000
	loadMoreID     = "__nvm:load-more"
	retryID        = "__nvm:retry-load"
	failedID       = "__nvm:load-failed"
)

// start runs what a freshly opened view does: its data source loads (or its snapshot shows), its immediate refresh
// runs, and its interval refresh starts. It runs once.
func (s *Session) start() {
	if s.started {
		return
	}
	s.started = true
	if s.deps.Cache != nil && s.view.Source != nil {
		s.sourceLease = s.deps.Cache.acquire(s.extID, s.installationIdentity)
	}
	s.immediate = s.view.Refresh != nil && s.view.Refresh.Immediate
	s.startInterval()
	if s.view.Source != nil {
		s.loadSource()
		return
	}
	s.afterFirstLoad()
}

// loadSource shows the snapshot of a data source when it has one, and loads the items again when the snapshot is
// stale or missing.
func (s *Session) loadSource() {
	source := s.view.Source
	if key := s.snapshotKey(); key != "" {
		if entry, epoch, ok := s.sourceLease.get(key); ok {
			s.sourceEpoch = epoch
			s.items = entry.Items
			age := s.deps.Clock.Now().Sub(entry.At)
			if age < durationMs(source.TTLMs, defaultFreshMs) {
				s.afterFirstLoad()
				return
			}
			if age < durationMs(source.StaleTTLMs, defaultStaleMs) {
				s.reload()
				return
			}
			s.items = nil
		}
	}
	s.reload()
}

// reload runs the data source's loader off the UI thread. Items it returns replace the view's items and its snapshot;
// a failure keeps the items that were showing and says why.
func (s *Session) reload() {
	if s.deps.Load == nil && s.deps.LoadContext == nil {
		s.failure = &api.Failure{Code: api.CodeUnavailable, Message: "Data loading is not available in this build."}
		return
	}
	if s.sourceLoading {
		s.sourcePending = true
		return
	}
	s.sourceLoading = true
	s.itemsRevision++
	revision := s.itemsRevision
	var generation uint64
	if s.sourceLease != nil {
		generation = s.sourceLease.version()
	}
	s.loading, s.failure = true, nil
	done := func(items []api.Item, failure *api.Failure) {
		s.sourceLoading = false
		if s.dismissed || !s.currentInstallation() || revision != s.itemsRevision {
			return
		}
		if s.sourcePending || (s.sourceLease != nil && s.sourceLease.version() != generation) {
			s.sourcePending = false
			s.reload()
			return
		}
		s.loading = false
		if failure != nil {
			s.failure = failure
			s.refreshHost()
			return
		}
		if key := s.snapshotKey(); key != "" && generation != 0 {
			if s.sourceLease.put(key, generation, CacheEntry{Items: items, At: s.deps.Clock.Now()}) == sourceStale {
				s.reload()
				return
			}
		}
		s.items = items
		s.sourceEpoch = generation
		s.refreshHost()
		s.afterFirstLoad()
	}
	if s.deps.LoadContext != nil {
		s.deps.LoadContext(s.workContext, s.extID, s.view.Source.Loader, s.installationIdentity, done)
	} else {
		s.deps.Load(s.extID, s.view.Source.Loader, done)
	}
}

// afterFirstLoad runs the immediate refresh once the first items are on screen, so the refresh patches what is shown.
func (s *Session) afterFirstLoad() {
	if s.immediate {
		s.immediate = false
		s.refreshNow()
	}
}

// refreshNow runs the view's refresh handler when none is in flight, and patches the view with its result. A chat view
// takes a whole new view instead, since its transcript, tier and side page change together while a reply streams.
func (s *Session) refreshNow() {
	refresh := s.view.Refresh
	if refresh == nil || refresh.Ref == nil || s.refreshing || s.dismissed {
		return
	}
	s.refreshing = true
	var generation uint64
	if s.sourceLease != nil {
		generation = s.sourceLease.version()
	}
	var snapshotVersion uint64
	if s.snapshotRef != nil && s.deps.CommandSnapshots != nil {
		snapshotVersion = s.deps.CommandSnapshots.Version(s.snapshotIdentity, *s.snapshotRef)
	}
	s.invokeRefresh(*refresh.Ref, func(result api.Result, failure *api.Failure) {
		s.refreshing = false
		if failure != nil && s.snapshotRef != nil && s.deps.CommandSnapshots != nil {
			s.deps.CommandSnapshots.RemoveVersion(s.snapshotIdentity, *s.snapshotRef, snapshotVersion)
		}
		if s.dismissed || !s.currentInstallation() {
			return
		}
		if s.sourceLease != nil && s.sourceLease.version() != generation {
			if s.refreshPending {
				s.refreshPending = false
				s.refreshNow()
			}
			return
		}
		if s.snapshotRef != nil && s.deps.CommandSnapshotIdentity != nil && !sameSnapshotIdentity(s.snapshotIdentity, s.deps.CommandSnapshotIdentity(s.extID)) {
			return
		}
		if s.refreshPending && failure == nil {
			s.refreshPending = false
			s.refreshNow()
			return
		}
		manualHost := s.manualRefreshHost
		s.manualRefreshHost = nil
		if failure != nil && manualHost != nil {
			manualHost.Toast(failure.Message)
		}
		if failure == nil && result.View != nil && result.View.Kind == api.ViewChat && result.View.Chat != nil {
			s.view = *result.View
			s.refreshHost()
			return
		}
		if failure != nil {
			s.refreshPending = false
			if failure.Code == api.CodePermission && snapshotEvidenceDeniesPaths(result.SnapshotEvidence, s.snapshotSourcePaths) {
				s.clearPrivateSnapshot(failure)
			}
			return
		}
		if len(s.snapshotSourcePaths) > 0 {
			filterSnapshotDenied(&result, s.snapshotStaleHashes)
		}
		s.snapshotStaleHashes = nil
		s.restoreSnapshotPresentation(result)
		if result.View != nil {
			s.snapshotSourcePaths = verifiedSnapshotPaths(result.View.Items, result.SnapshotEvidence)
		}
		if result.Patch == nil && result.View != nil && result.View.Kind == s.view.Kind && len(s.sections) == 0 && len(result.View.Sections) == 0 && (s.view.Kind == api.ViewGrid || s.view.Kind == api.ViewList) {
			v := result.View
			mode := refresh.Mode
			if mode == "" {
				mode = api.PatchReplace
			}
			result.Patch = &api.ViewPatch{Mode: mode, Items: v.Items, IsLoading: &v.IsLoading, EmptyView: v.EmptyView, Status: v.Status}
		}
		if result.Patch == nil {
			if s.snapshotRef != nil && s.deps.CommandSnapshots != nil {
				s.deps.CommandSnapshots.RemoveVersion(s.snapshotIdentity, *s.snapshotRef, snapshotVersion)
			}
			return
		}
		patch := *result.Patch
		if patch.Mode == "" {
			patch.Mode = refresh.Mode
		}
		if patch.Mode == api.PatchReplace {
			s.itemsRevision++
			s.loading, s.failure = false, nil
		}
		s.Apply(patch)
		s.rememberVerifiedSnapshot(result.SnapshotEvidence)
		s.cacheRefreshPatch(patch, generation)
		s.refreshHost()
	})
}

// startInterval runs the refresh every IntervalMs while the view is open, on the UI thread.
func (s *Session) startInterval() {
	interval := s.refreshInterval()
	if interval <= 0 || s.deps.Clock == nil {
		return
	}
	stop := make(chan struct{})
	s.stopInterval = stop
	ticker := s.deps.Clock.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C():
				s.onUI(s.refreshNow)
			}
		}
	}()
}

// close ends the view's background work when the palette leaves it: no load or refresh result changes it afterwards.
func (s *Session) close() {
	if s.dismissed {
		return
	}
	s.dismissed = true
	if s.sourceLease != nil {
		s.sourceLease.release()
	}
	if s.cancelImages != nil {
		s.cancelImages()
	}
	if s.cancelWork != nil {
		s.cancelWork()
	}
	if s.stopInterval != nil {
		close(s.stopInterval)
	}
}

// loadMoreRow is the pagination row at the end of the list. Its action asks the extension for the next page.
func (s *Session) loadMoreRow() search.Item {
	next := *s.view.Pagination.Next
	return search.Item{
		ID: loadMoreID, Title: "Load more", Icon: "plus", Tone: extensionTone,
		Actions: []search.Action{{Title: "Load more", Dismiss: search.DismissNever, Run: func(h search.Host) { s.loadMore(h, next) }}},
	}
}

// loadMore appends the next page. An empty page, or one without a patch, is the end of the list.
func (s *Session) loadMore(h search.Host, next api.ActionRef) {
	revision := s.itemsRevision
	s.invokeRaw(next, "", func(result api.Result, failure *api.Failure) {
		if s.dismissed || revision != s.itemsRevision {
			return
		}
		if failure != nil {
			h.Toast(failure.Message)
			return
		}
		if result.Patch == nil || len(result.Patch.Items) == 0 {
			s.hasMore = false
			h.Refresh()
			return
		}
		patch := *result.Patch
		if patch.Mode == "" {
			patch.Mode = api.PatchAppend
		}
		s.Apply(patch)
		h.Refresh()
	})
}

// failureRows is what a failed load shows above the items it kept: the reason, and a retry row when the source allows it.
func (s *Session) failureRows() []search.Item {
	rows := []search.Item{{
		ID: failedID, Title: "Could not load these items", Subtitle: s.failure.Message,
		Contextual: true, Disabled: true,
	}}
	if s.view.Source != nil && s.view.Source.Retry {
		rows = append(rows, search.Item{
			ID: retryID, Title: "Try again", Icon: "rotate-ccw", Tone: extensionTone,
			Actions: []search.Action{{Title: "Try again", Dismiss: search.DismissNever, Run: func(search.Host) { s.reload() }}},
		})
	}
	return rows
}

// invokeRaw runs one handler of the view and passes its result to done on the UI thread. A view that is closed never
// runs a handler.
func (s *Session) invokeRaw(ref api.ActionRef, value string, done func(api.Result, *api.Failure)) {
	if s.dismissed {
		return
	}
	if s.deps.Invoke == nil {
		done(api.Result{}, &api.Failure{Code: api.CodeUnavailable, Message: unavailableMessage})
		return
	}
	s.deps.Invoke(s.extID, api.Invocation{Ref: ref, Value: value}, done)
}

// invokeInPlace runs a handler whose result changes the open view without closing the palette, such as a picker.
func (s *Session) invokeInPlace(ref api.ActionRef, value string) {
	s.invokeRaw(ref, value, func(result api.Result, failure *api.Failure) {
		if failure != nil {
			s.toast(failure.Message)
			return
		}
		if result.Patch != nil {
			s.Apply(*result.Patch)
		}
		if view := result.View; view != nil && result.Navigation == api.NavReplace && view.Kind == s.view.Kind {
			s.replaceInPlace(*view)
		}
		s.refreshHost()
	})
}

func (s *Session) toast(message string) {
	if s.host != nil {
		s.host.Toast(message)
	}
}

func (s *Session) refreshHost() {
	if s.host == nil {
		return
	}
	if host, ok := s.host.(interface {
		RefreshTop(func(*palette.View) bool)
	}); ok {
		page := s.palettePage
		host.RefreshTop(func(current *palette.View) bool { return current == page })
		return
	}
	s.host.Refresh()
}

// onUI runs fn on the UI thread through the host's hook, or in place when there is none.
func (s *Session) onUI(fn func()) {
	if s.deps.OnUI == nil {
		fn()
		return
	}
	s.deps.OnUI(fn)
}

// snapshotKey names the view's snapshot in the cache, or "" when the view keeps none.
func (s *Session) snapshotKey() string {
	source := s.view.Source
	if source == nil || source.CacheKey == "" || s.sourceLease == nil || s.deps.Clock == nil {
		return ""
	}
	return source.CacheKey
}

func durationMs(ms, fallback int64) time.Duration {
	if ms <= 0 {
		ms = fallback
	}
	return time.Duration(ms) * time.Millisecond
}

// chatTickInterval is how often a chat view checks for a new transcript. It runs whenever the chat is open, so a reply
// that starts after the view opened shows its words; a view with no refresh does nothing on a tick.
const chatTickInterval = 150 * time.Millisecond

// refreshInterval is the period of the view's refresh ticks: a chat view ticks at chatTickInterval, and any other view
// at its own IntervalMs, or not at all when it has no refresh.
func (s *Session) refreshInterval() time.Duration {
	if s.view.Kind == api.ViewChat {
		return chatTickInterval
	}
	if s.view.Refresh == nil || s.view.Refresh.IntervalMs <= 0 {
		return 0
	}
	return time.Duration(s.view.Refresh.IntervalMs) * time.Millisecond
}
