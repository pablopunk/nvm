import {
  getNevermindAuth,
  signInToNevermind,
  signOutFromNevermind,
} from '../nevermind-auth';
import { extensionContext } from './_context';

export function createAccountExtension() {
  const extensionId = 'nevermind.account';

  async function accountItem() {
    const existing = await getNevermindAuth();
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

  return {
    id: extensionId,
    title: 'Nevermind Account',
    permissions: [] as const,
    searchItems: async () => [await accountItem()],
    rootItems: async () => [await accountItem()],
  };
}
