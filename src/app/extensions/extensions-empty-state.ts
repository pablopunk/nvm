const SIGNED_OUT_ACCOUNT_ITEM_ID = 'account-login';

export const SIGNED_IN_EXTENSIONS_EMPTY_VIEW = {
  title: 'No extensions yet',
  subtitle:
    'Type what you want to automate in the palette and choose Automate to build one with AI.',
};

export interface ExtensionsEmptyAccountItem<Action> {
  id?: string;
  primaryAction?: Action;
}

function signInToBuildExtensionsItem<Action>(
  account: ExtensionsEmptyAccountItem<Action>,
) {
  return {
    id: 'extensions-sign-in',
    title: 'Sign in to build extensions',
    subtitle:
      'Connect your Nevermind account to build extensions with AI. They will appear here.',
    icon: 'person',
    primaryAction: account.primaryAction,
  };
}

export function extensionsEmptyState<Action>(
  account: ExtensionsEmptyAccountItem<Action> | null | undefined,
) {
  if (account?.id === SIGNED_OUT_ACCOUNT_ITEM_ID)
    return {
      items: [signInToBuildExtensionsItem(account)],
      emptyView: SIGNED_IN_EXTENSIONS_EMPTY_VIEW,
    };
  return { items: [], emptyView: SIGNED_IN_EXTENSIONS_EMPTY_VIEW };
}
