package app

// invalidateExtensionViews serves an extension's Views.Invalidate: the data-source snapshots of the installation that
// began the invocation are dropped, so the next open loads fresh rows, and its visible view refreshes now. A call without
// an invocation origin drops nothing it cannot name. The call arrives on the child's exchange goroutine, and
// NotifyTopSourceInvalidated is safe from any goroutine, so it does not wait.
func (p *App) invalidateExtensionViews(extensionID string, identity any) {
	extensionSnapshots.InvalidateExtension(extensionSnapshots.Scope(extensionID, identity))
	p.Palette.NotifyTopSourceInvalidated(extensionID, identity)
}
