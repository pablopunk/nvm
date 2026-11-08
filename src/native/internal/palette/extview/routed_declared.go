package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// runRoutedDeclared keeps a result's installation provenance on host-owned actions.
func (s *Session) runRoutedDeclared(h search.Host, r route) {
	if r.identityBound && s.deps.CommandSnapshotIdentity != nil {
		current := s.deps.CommandSnapshotIdentity(r.extID)
		if (r.identity != nil || current != nil) && !sameSnapshotIdentity(r.identity, current) {
			h.Toast(unavailableMessage)
			return
		}
	}
	switch r.action.Kind {
	case api.ActionPushView, api.ActionReplaceView, api.ActionRootView:
		if r.action.View == nil || r.action.View.Validate() != nil {
			h.Toast(unavailableMessage)
			return
		}
		nav := api.NavPush
		if r.action.Kind == api.ActionReplaceView {
			nav = api.NavReplace
		}
		if r.action.Kind == api.ActionRootView {
			nav = api.NavRoot
		}
		s.finishAt(h, r.extID, r.action.Title, api.Result{View: r.action.View, Navigation: nav, OriginIdentity: r.identity, OriginBound: r.identityBound}, nil, r.depth)
	case api.ActionToggleSetting:
		s.toggleDeclaredSetting(h, r)
	case api.ActionWindow:
		if r.identityBound && s.deps.WindowBound != nil {
			h.Hide()
			s.deps.WindowBound(r.extID, r.identity, r.action)
			return
		}
		if s.deps.Window == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		s.deps.Window(r.extID, r.action)
	case api.ActionSystem:
		if r.action.Op == "open-system-settings" && r.action.Setting != "" && s.deps.OpenSettingsPane != nil {
			h.Hide()
			s.deps.OpenSettingsPane(r.action.Setting)
		} else {
			s.runDeclared(h, r.action)
		}
	case api.ActionOpenPath, api.ActionRevealPath, api.ActionSetQuery, api.ActionQuickLook, api.ActionOpenWith, api.ActionTrash:
		s.runDeclared(h, r.action)
	case api.ActionRefreshView, api.ActionCamera:
		s.action(r.action, "").Run(h)
	default:
		s.actionBodyFor(r, "").Run(h)
	}
}
