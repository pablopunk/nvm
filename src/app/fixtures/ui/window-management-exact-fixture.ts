import { exactWindowPromptView } from '../../extensions/window-management-exact-commands';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function windowManagementMovePromptFixtureView(ctx: ExtensionContext) {
  return exactWindowPromptView(ctx, 'move', {
    values: { unit: 'percent', x: '25', y: '' },
  });
}

export function windowManagementResizeErrorFixtureView(ctx: ExtensionContext) {
  return exactWindowPromptView(ctx, 'resize', {
    values: { unit: 'points', width: '-20', height: '0' },
    errors: {
      width: 'Enter a number above 0, up to 100000.',
      height: 'Enter a number above 0, up to 100000.',
    },
  });
}
