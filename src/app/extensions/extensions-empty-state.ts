const SIGNED_OUT_ACCOUNT_ITEM_ID = 'account-login';

export const SIGNED_IN_EXTENSIONS_EMPTY_VIEW = {
  title: 'No extensions yet',
  subtitle:
    'Type what you want to automate in the palette and choose Automate to build one with AI.',
};

export interface ExtensionsEmptyAccountItem {
  id?: string;
  primaryAction?: ExtensionAction;
}

function signInToBuildExtensionsItem(account: ExtensionsEmptyAccountItem) {
  return {
    id: 'extensions-sign-in',
    title: 'Sign in to build extensions',
    subtitle:
      'Connect your Nevermind account to build extensions with AI. They will appear here.',
    icon: 'person',
    primaryAction: account.primaryAction,
  };
}

export function extensionsEmptyState(
  account: ExtensionsEmptyAccountItem | null | undefined,
) {
  if (account?.id === SIGNED_OUT_ACCOUNT_ITEM_ID)
    return {
      items: [signInToBuildExtensionsItem(account)],
      emptyView: SIGNED_IN_EXTENSIONS_EMPTY_VIEW,
    };
  return { items: [], emptyView: SIGNED_IN_EXTENSIONS_EMPTY_VIEW };
}
import type { ExtensionAction } from '../resources/nevermind-extension-api';
