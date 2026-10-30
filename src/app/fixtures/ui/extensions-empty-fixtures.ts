import { extensionsEmptyState } from '../../extensions/extensions-empty-state';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

function fixtureSignInAction(ctx: ExtensionContext) {
  return ctx.actions.push('Log in', {
    type: 'preview',
    title: 'Fixture sign-in',
    content:
      'The real Extensions view runs the Nevermind account sign-in flow.',
  });
}

export function signedOutExtensionsEmptyView(ctx: ExtensionContext) {
  const state = extensionsEmptyState({
    id: 'account-login',
    primaryAction: fixtureSignInAction(ctx),
  });
  return ctx.ui.list({
    id: 'dev-ui-extensions-empty-signed-out',
    title: 'Dev UI · Extensions Empty (Signed Out)',
    emptyView: state.emptyView,
    items: state.items,
  });
}

export function signedInExtensionsEmptyView(ctx: ExtensionContext) {
  const state = extensionsEmptyState({ id: 'account-logout' });
  return ctx.ui.list({
    id: 'dev-ui-extensions-empty-signed-in',
    title: 'Dev UI · Extensions Empty (Signed In)',
    emptyView: state.emptyView,
    items: state.items,
  });
}
