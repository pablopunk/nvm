package services

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxKeptReviews bounds the uninstall reviews the host keeps. The oldest is dropped first, so a token the palette still
// holds stays valid for as long as the user can reach its review.
const maxKeptReviews = 8

// uninstallReviews keeps the reviewed snapshots by token. Trash runs the snapshot that was reviewed, not a fresh
// discovery, so the user confirms exactly the items the review showed.
type uninstallReviews struct {
	mu      sync.Mutex
	byToken map[string]appfeature.UninstallSnapshot
	order   []string
	next    int
}

func newUninstallReviews() *uninstallReviews {
	return &uninstallReviews{byToken: map[string]appfeature.UninstallSnapshot{}}
}

// keep stores a snapshot and returns the token that names it.
func (r *uninstallReviews) keep(snapshot appfeature.UninstallSnapshot) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	token := "uninstall-" + strconv.Itoa(r.next)
	r.byToken[token] = snapshot
	r.order = append(r.order, token)
	for len(r.order) > maxKeptReviews {
		delete(r.byToken, r.order[0])
		r.order = r.order[1:]
	}
	return token
}

func (r *uninstallReviews) lookup(token string) (appfeature.UninstallSnapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, ok := r.byToken[token]
	return snapshot, ok
}

// serveUninstall answers the two uninstall operations of desktop.apps.
func (h *Handler) serveUninstall(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpUninstallReview:
		return h.uninstallReview(call.Args)
	case protocol.OpUninstallTrash:
		return h.uninstallTrash(call.Args)
	}
	return nil, invalid("desktop.apps has no operation %q", call.Op)
}

// uninstallReview discovers what uninstalling one indexed app would move, and keeps the review under a token.
func (h *Handler) uninstallReview(raw json.RawMessage) (api.UninstallReview, error) {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.UninstallReview{}, err
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppUninstall, "uninstall")
	if err != nil {
		return api.UninstallReview{}, err
	}
	app, err := h.indexedApp(args.Path)
	if err != nil {
		return api.UninstallReview{}, err
	}
	discovery := lifecycle.DiscoverUninstall(app.Path)
	review := api.UninstallReview{Ready: discovery.Ready, Message: discovery.Message, Notes: noteMessages(discovery.Notes)}
	if !discovery.Ready {
		return review, nil
	}
	review.Token = h.reviews.keep(discovery.Snapshot)
	for _, candidate := range discovery.Snapshot.Candidates {
		review.Items = append(review.Items, api.UninstallItem{
			ID: candidate.ID, Path: candidate.Path, Kind: candidate.Kind, Reason: candidate.Reason,
			Selected: candidate.SelectedByDefault,
		})
	}
	return review, nil
}

// uninstallTrash moves the selected items of a kept review to the Trash, under the rate limit for app actions.
func (h *Handler) uninstallTrash(raw json.RawMessage) (api.UninstallResult, error) {
	var args protocol.UninstallTrashArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.UninstallResult{}, err
	}
	lifecycle, err := h.lifecycle(platform.CapabilityAppUninstall, "uninstall")
	if err != nil {
		return api.UninstallResult{}, err
	}
	snapshot, ok := h.reviews.lookup(args.Token)
	if !ok {
		return api.UninstallResult{}, notFound("this uninstall review has expired; review the app again")
	}
	if !h.actions.allow() {
		return api.UninstallResult{}, rateExceeded("app actions")
	}
	values := make(map[string]bool, len(args.IDs))
	for _, id := range args.IDs {
		values[id] = true
	}
	result := lifecycle.TrashUninstall(snapshot, values)
	return uninstallResultOf(result, lifecycle.TrashFolder()), nil
}

func uninstallResultOf(result appfeature.TrashResult, trashFolder string) api.UninstallResult {
	out := api.UninstallResult{Status: result.Status, TrashFolder: trashFolder, Moved: result.Moved}
	for _, item := range result.Untouched {
		out.Untouched = append(out.Untouched, api.UninstallUntouched{Path: item.Path, Message: item.Message})
	}
	return out
}

func noteMessages(notes []appfeature.Note) []string {
	var out []string
	for _, note := range notes {
		out = append(out, note.Message)
	}
	return out
}
