export type ActionMenuSurfaceKind =
  | 'actions'
  | 'confirmation'
  | 'submenu'
  | 'prompt';

export function actionMenuPresentation(kind: ActionMenuSurfaceKind) {
  if (kind === 'confirmation' || kind === 'prompt') {
    return 'default';
  }
  return 'compact';
}
