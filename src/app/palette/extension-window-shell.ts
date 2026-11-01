/** Popovers open under a menu bar icon and need a panel behind native views. */
export function extensionWindowShellClassName(
  windowOptions: Record<string, unknown>,
  isLoading: boolean,
) {
  const frameless = windowOptions.chrome === 'none';
  return [
    'extensionWindowShell',
    isLoading ? 'loadingBorder' : '',
    windowOptions.titleBar === 'hidden' && !frameless
      ? 'extensionWindowTitleBarHidden'
      : '',
    frameless ? 'extensionWindowChromeNone' : '',
    windowOptions.position === 'menu-bar' ? 'extensionWindowPopover' : '',
  ]
    .filter(Boolean)
    .join(' ');
}
