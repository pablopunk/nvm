package windowmgmt

import "context"

// restorePaletteFocus hands focus back to the app the palette was opened from. The command only runs
// when that app is still in front afterwards, so no window is moved in the wrong app.
func (s *Service) restorePaletteFocus(ctx context.Context, target Target) *Failure {
	if s.focus == nil {
		return newFailure(ReasonFocusNotRestored, messageFocusChangedPalette)
	}
	paletteTarget, ok := s.focus.PaletteFocusTarget()
	if !ok || paletteTarget.PID != target.PID {
		return newFailure(ReasonFocusNotRestored, messageFocusChangedPalette)
	}
	if s.focus.HidePaletteAndRestoreFocus(ctx, target) && s.isFrontmost(ctx, target.PID) {
		return nil
	}
	return newFailure(ReasonFocusNotRestored, messageFocusNotRestored)
}

// guardShortcutFocus refuses a shortcut command when the target app is no longer in front. A shortcut
// never takes focus itself.
func (s *Service) guardShortcutFocus(ctx context.Context, target Target) *Failure {
	if s.isFrontmost(ctx, target.PID) {
		return nil
	}
	return newFailure(ReasonFocusNotRestored, messageFocusChangedKey)
}

func (s *Service) isFrontmost(ctx context.Context, pid int) bool {
	app, found, err := s.windows.Frontmost(ctx)
	return err == nil && found && app.PID == pid
}
