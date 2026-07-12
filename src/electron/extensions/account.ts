import {
  getNevermindAuth,
  signInToNevermind,
  signOutFromNevermind,
} from '../nevermind-auth';
import { getByoKey, clearByoKey } from '../byo-key';
import { extensionContext } from './_context';

export function createAccountExtension() {
  const extensionId = 'nevermind.account';

  async function accountItem() {
    const existing = await getNevermindAuth();
    const byo = await getByoKey();
    if (existing) {
      return {
        id: 'account-logout',
        actionId: 'account-logout',
        title: 'Log out of Nevermind',
        subtitle: `Signed in as ${existing.email}`,
        icon: 'person',
        score: 18,
        aliases: ['logout', 'sign out', 'nevermind', 'account', 'disconnect'],
        primaryAction: {
          type: 'runExtensionAction',
          title: 'Log out',
          __handler: async () => {
            const { revoked } = await signOutFromNevermind();
            extensionContext.setActiveNevermindBaseUrl(null);
            await extensionContext.nevermindAi?.disposeAllSessions?.();
            extensionContext.invalidateExtensionRootItems();
            extensionContext.broadcastAuthChanged({ authed: false });
            const suffix = revoked
              ? ''
              : ' (token revoke failed - check connection)';
            return {
              toast: {
                message: `Logged out of ${existing.email}${suffix}`,
                tone: revoked ? ('default' as const) : ('error' as const),
              },
            };
          },
        },
      };
    }
    return {
      id: 'account-login',
      actionId: 'account-login',
      title: 'Log in to Nevermind',
      subtitle: 'Connect this device to your Nevermind account',
      icon: 'person',
      score: 18,
      aliases: ['login', 'sign in', 'nevermind', 'account', 'connect'],
      primaryAction: {
        type: 'runExtensionAction',
        title: 'Log in',
        __handler: async () => {
          const result = await signInToNevermind();
          extensionContext.invalidateExtensionRootItems();
          if (result.ok)
            extensionContext.broadcastAuthChanged({
              authed: true,
              email: result.auth.email,
            });
          const message = result.ok
            ? `Logged in as ${result.auth.email}`
            : `Log-in failed: ${'error' in result ? result.error : 'unknown'}`;
          return {
            toast: {
              message,
              tone: result.ok ? ('default' as const) : ('error' as const),
            },
          };
        },
      },
    };
  }

  async function byoKeyItem() {
    const byo = await getByoKey();
    if (byo) {
      return {
        id: 'byo-key-clear',
        actionId: 'byo-key-clear',
        title: 'Clear BYO provider key',
        subtitle: `Using own key for ${byo.provider} (${byo.modelName})`,
        icon: 'key',
        score: 4,
        aliases: ['byo', 'own key', 'clear key'],
        primaryAction: {
          type: 'runExtensionAction',
          title: 'Clear BYO Key',
          __handler: async () => {
            await clearByoKey();
            extensionContext.invalidateExtensionRootItems();
            return {
              toast: {
                message: 'BYO provider key cleared. Using Nevermind backend.',
                tone: 'default' as const,
              },
            };
          },
        },
      };
    }
    return null;
  }

  return {
    id: extensionId,
    title: 'Nevermind Account',
    permissions: [] as const,
    searchItems: async () => {
      const items = [await accountItem()];
      const byo = await byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
    rootItems: async () => {
      const items = [await accountItem()];
      const byo = await byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
  };
}
