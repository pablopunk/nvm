// biome-ignore-all lint: This extension follows the existing palette-extension API conventions.
import { clearByoKey, getCachedByoKey } from '../electron/byo-key';
import {
  getCachedNevermindAuth,
  signOutFromNevermind,
} from '../electron/nevermind-auth';
import { extensionContext } from './_context';

export function createAccountExtension() {
  const extensionId = 'nevermind.account';

  function accountItem() {
    const existing = getCachedNevermindAuth();
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
      environment: 'development' | 'production' | 'pr_preview' | 'custom';
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
    const developmentAction = {
      type: 'runExtensionAction',
      title: 'Development',
      __handler: async () => switchBackend({ environment: 'development' }),
    };
    const productionAction = {
      type: 'runExtensionAction',
      title: 'Production',
      __handler: async () =>
        switchBackend({
          environment: 'production',
        }),
    };
    const previewAction = {
      type: 'promptAction',
      title: 'Preview URL…',
      fields: [
        {
          id: 'baseUrl',
          type: 'text',
          label: 'Preview URL',
          placeholder: 'https://nvm-your-branch.vercel.app',
          required: true,
        },
      ],
      targetAction: {
        type: 'runExtensionAction',
        title: 'Use Preview',
        __handler: async (
          _ctx: unknown,
          action: { formValues?: { baseUrl?: string } },
        ) =>
          switchBackend({
            environment: 'pr_preview',
            baseUrl: action.formValues?.baseUrl,
          }),
      },
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
        ...(!extensionContext.isPackaged
          ? [
              {
                id: 'account-switch-backend-development',
                title: 'Development',
                subtitle: 'http://localhost:4321',
                primaryAction: developmentAction,
              },
            ]
          : []),
        {
          id: 'account-switch-backend-preview',
          title: 'Preview URL…',
          subtitle: 'Use a Vercel Preview deployment',
          primaryAction: previewAction,
        },
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
      subtitle: 'Switch Development, Preview, Production, or custom backend',
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
        sections: [
          {
            actions: [
              ...(!extensionContext.isPackaged ? [developmentAction] : []),
              previewAction,
              productionAction,
              customAction,
            ],
          },
        ],
      },
    };
  }

  function backendStatusItem() {
    const status = extensionContext.getNevermindDebugStatus();
    const backend = status.backend
      ? `${status.backend.environment} (${status.backend.version})`
      : 'unavailable';
    const serverEnvironment =
      status.backend?.environment === 'preview'
        ? 'pr_preview'
        : status.backend?.environment;
    const mismatch =
      Boolean(status.backend) &&
      serverEnvironment !== status.active.environment;
    return {
      id: 'account-backend-status',
      actionId: 'account-backend-status',
      title: `Nevermind: Backend Status${mismatch ? ' — mismatch' : ''}`,
      subtitle: `Client ${status.client.environment} · Active ${status.active.environment} · ${status.active.baseUrl} · Server ${backend}`,
      icon: mismatch ? 'warning' : 'globe',
      score: 6,
      aliases: ['debug', 'backend status', 'environment status'],
    };
  }

  function byoKeyItem() {
    const byo = getCachedByoKey();
    if (!byo) return null;
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

  return {
    id: extensionId,
    title: 'Nevermind Account',
    capabilities: [] as const,
    searchItems: () => {
      const items = [
        accountItem(),
        backendEnvironmentItem(),
        backendStatusItem(),
      ];
      const byo = byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
    rootItems: () => {
      const items = [
        accountItem(),
        backendEnvironmentItem(),
        backendStatusItem(),
      ];
      const byo = byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
  };
}
