// biome-ignore-all lint: This extension follows the existing palette-extension API conventions.
import { getNevermindAuth, signOutFromNevermind } from '../nevermind-auth';
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
          const result = await extensionContext.signInToNevermind();
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

  function backendEnvironmentItem() {
    async function switchBackend(input: {
      environment: 'production' | 'pr_preview' | 'custom';
      baseUrl?: string;
    }) {
      const result =
        await extensionContext.switchNevermindBackendEnvironment(input);
      return {
        toast: {
          message: result.message,
          tone: result.ok ? ('default' as const) : ('error' as const),
        },
      };
    }
    const productionAction = {
      type: 'runExtensionAction',
      title: 'Production',
      __handler: async () =>
        switchBackend({
          environment: 'production',
        }),
    };
    const customAction = {
      type: 'promptAction',
      title: 'Custom URL…',
      fields: [
        {
          id: 'baseUrl',
          type: 'text',
          label: 'Backend URL',
          placeholder: 'https://your-preview.vercel.app',
          required: true,
        },
      ],
      targetAction: {
        type: 'runExtensionAction',
        title: 'Use Custom URL',
        __handler: async (
          _ctx: unknown,
          action: { formValues?: { baseUrl?: string } },
        ) =>
          switchBackend({
            environment: 'custom',
            baseUrl: action.formValues?.baseUrl,
          }),
      },
    };
    const choicesView = {
      type: 'list',
      title: 'Switch Backend Environment',
      searchBarPlaceholder: 'Choose an environment',
      items: [
        {
          id: 'account-switch-backend-production',
          title: 'Production',
          subtitle: 'https://api.nvm.fyi',
          primaryAction: productionAction,
        },
        {
          id: 'account-switch-backend-custom',
          title: 'Custom URL…',
          subtitle: 'Use a validated HTTPS backend URL',
          primaryAction: customAction,
        },
      ],
    };
    return {
      id: 'account-switch-backend',
      actionId: 'account-switch-backend',
      title: 'Nevermind: Switch Backend Environment',
      subtitle: 'Choose Production or a validated custom backend URL',
      icon: 'globe',
      score: 20,
      aliases: [
        'backend',
        'environment',
        'production',
        'preview',
        'custom url',
      ],
      primaryAction: {
        type: 'pushView',
        title: 'Choose Backend Environment',
        view: choicesView,
      },
      actionPanel: {
        sections: [{ actions: [productionAction, customAction] }],
      },
    };
  }

  return {
    id: extensionId,
    title: 'Nevermind Account',
    permissions: [] as const,
    searchItems: async () => [await accountItem(), backendEnvironmentItem()],
    rootItems: async () => [await accountItem(), backendEnvironmentItem()],
  };
}
