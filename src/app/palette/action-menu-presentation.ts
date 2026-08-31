interface ActionMenuHostLayout {
  builderWorkspaceVisible: boolean;
  hasNavigatedChild: boolean;
  hasSiblingViews: boolean;
  isRootLikeView: boolean;
}

export type ActionMenuSurfaceKind =
  | 'actions'
  | 'confirmation'
  | 'submenu'
  | 'prompt';

export type ConfirmationReturnSurface = 'panel' | 'submenu' | 'view';

export function actionMenuPresentation(kind: ActionMenuSurfaceKind) {
  return kind === 'prompt' ? 'default' : 'compact';
}

export function confirmationReturnSurface(
  hasSubmenu: boolean,
  panelOpen: boolean,
): ConfirmationReturnSurface {
  if (hasSubmenu) {
    return 'submenu';
  }
  return panelOpen ? 'panel' : 'view';
}

export function actionMenuHostIsStacked({
  builderWorkspaceVisible,
  hasNavigatedChild,
  hasSiblingViews,
  isRootLikeView,
}: ActionMenuHostLayout) {
  return (
    (!builderWorkspaceVisible && hasNavigatedChild && !isRootLikeView) ||
    hasSiblingViews
  );
}
