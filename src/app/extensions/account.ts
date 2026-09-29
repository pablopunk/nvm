// biome-ignore-all lint: This extension follows the existing palette-extension API conventions.
import { clearByoKey, getCachedByoKey } from '../electron/byo-key';
import {
  getCachedNevermindAuth,
  signOutFromNevermind,
} from '../electron/nevermind-auth';
import type { NevermindDeviceSignInStatus } from '../shared/nevermind-auth';
import { extensionContext } from './_context';
import { showExtensionFeedback } from './feedback';

const LOGIN_INDICATOR_ID = 'account-login';
const deviceSignInViewId = 'nevermind-device-sign-in';
const deviceSignInStatusId = 'device-sign-in-status';
const deviceSignInUrlId = 'device-sign-in-url';
const deviceSignInCodeId = 'device-sign-in-code';
const deviceSignInRetryId = 'device-sign-in-retry';
const deviceSignInCancelId = 'device-sign-in-cancel';

function deviceSignInStatusTitle(status: NevermindDeviceSignInStatus) {
  switch (status.state) {
    case 'starting':
      return 'Preparing sign-in';
    case 'pending':
      return status.browserOpenFailed
        ? 'Browser could not be opened'
        : 'Waiting for approval';
    case 'approved':
      return 'Sign-in approved';
    case 'expired':
      return 'Sign-in code expired';
    case 'cancelled':
      return 'Sign-in cancelled';
    case 'failed':
      return 'Sign-in could not be started';
  }
}

function deviceSignInStatusSubtitle(status: NevermindDeviceSignInStatus) {
  switch (status.state) {
    case 'starting':
      return 'Requesting a one-time verification code…';
    case 'pending':
      return status.browserOpenFailed
        ? 'Open the verification URL manually, or retry opening it below.'
        : 'Approve this device in your browser. This view updates automatically.';
    case 'approved':
      return `Signed in as ${status.email}`;
    case 'expired':
      return 'This one-time code expired. Return to the account command to try again.';
    case 'cancelled':
      return 'The sign-in request was cancelled. Return to the account command to try again.';
    case 'failed':
      return status.message;
  }
}

function deviceSignInStatusItem(status: NevermindDeviceSignInStatus) {
  return {
    id: deviceSignInStatusId,
    title: deviceSignInStatusTitle(status),
    subtitle: deviceSignInStatusSubtitle(status),
    icon:
      status.state === 'pending' && status.browserOpenFailed
        ? 'warning'
        : status.state === 'approved'
          ? 'check'
          : 'clock',
    disabled: true,
  };
}

function deviceSignInUrlItem(ctx: any, status: NevermindDeviceSignInStatus) {
  if (status.state !== 'pending') return null;
  const copy = ctx.actions.copyText(
    status.verificationUrl,
    'Copy verification URL',
  );
  return {
    id: deviceSignInUrlId,
    title: 'Verification URL',
    subtitle: status.verificationUrl,
    primaryAction: copy,
    detail: {
      title: 'Verification URL',
      text: status.verificationUrl,
      actions: [copy],
    },
  };
}

function deviceSignInCodeItem(ctx: any, status: NevermindDeviceSignInStatus) {
  if (status.state !== 'pending') return null;
  const copy = ctx.actions.copyText(status.code, 'Copy one-time user code');
  return {
    id: deviceSignInCodeId,
    title: 'One-time user code',
    subtitle: status.code,
    primaryAction: copy,
    detail: {
      title: 'One-time user code',
      text: status.code,
      actions: [copy],
    },
  };
}

function deviceSignInViewItems(ctx: any, status: NevermindDeviceSignInStatus) {
  const items: any[] = [deviceSignInStatusItem(status)];
  const urlItem = deviceSignInUrlItem(ctx, status);
  const codeItem = deviceSignInCodeItem(ctx, status);
  if (urlItem) items.push(urlItem);
  if (codeItem) items.push(codeItem);

  if (status.state === 'starting' || status.state === 'pending') {
    items.push(
      {
        id: deviceSignInRetryId,
        title: 'Open verification page',
        subtitle: 'Retry opening the sign-in page in your browser',
        primaryAction: ctx.actions.run(
          'Retry opening browser',
          async (actionCtx: any) => {
            const opened =
              await extensionContext.retryNevermindDeviceSignInBrowser();
            showExtensionFeedback(
              actionCtx,
              'Nevermind Account',
              opened
                ? 'Verification page opened.'
                : 'Browser could not be opened. Use the URL and code above.',
              opened ? 'success' : 'error',
            );
          },
        ),
      },
      {
        id: deviceSignInCancelId,
        title: 'Cancel sign-in',
        subtitle: 'Stop waiting for device approval',
        primaryAction: ctx.actions.run('Cancel sign-in', (actionCtx: any) => {
          const cancelled = extensionContext.cancelNevermindDeviceSignIn();
          if (!cancelled)
            showExtensionFeedback(
              actionCtx,
              'Nevermind Account',
              'There is no sign-in request to cancel.',
              'error',
            );
        }),
      },
    );
  }

  return items;
}

function deviceSignInView(ctx: any, status: NevermindDeviceSignInStatus) {
  const items = deviceSignInViewItems(ctx, status);
  if (
    status.state === 'expired' ||
    status.state === 'cancelled' ||
    status.state === 'failed'
  ) {
    items.push({
      id: 'device-sign-in-again',
      title: 'Start a new sign-in',
      primaryAction: ctx.actions.run('Start a new sign-in', startDeviceSignIn),
    });
  }
  return ctx.ui.list({
    id: deviceSignInViewId,
    title: 'Sign in to Nevermind',
    subtitle: deviceSignInStatusSubtitle(status),
    contentSizing: 'fit',
    ...(status.state === 'pending'
      ? { selectedItemId: deviceSignInUrlId }
      : {}),
    detail: { visible: true, placement: 'bottom' },
    items,
  });
}

function patchDeviceSignInView(ctx: any, status: NevermindDeviceSignInStatus) {
  const terminal =
    status.state === 'approved' ||
    status.state === 'expired' ||
    status.state === 'cancelled' ||
    status.state === 'failed';
  const items: any[] = [deviceSignInStatusItem(status)];
  if (!terminal) {
    const urlItem = deviceSignInUrlItem(ctx, status);
    const codeItem = deviceSignInCodeItem(ctx, status);
    if (urlItem) items.push(urlItem);
    if (codeItem) items.push(codeItem);
  } else if (status.state !== 'approved') {
    items.push({
      id: 'device-sign-in-again',
      title: 'Start a new sign-in',
      subtitle: 'Return to the account command to request a new code',
      primaryAction: {
        type: 'setSearchQuery',
        title: 'Log in to Nevermind',
        query: 'Log in to Nevermind',
      },
    });
  }
  extensionContext.patchOpenView(deviceSignInViewId, {
    mode: 'patch',
    items,
    ...(status.state === 'pending'
      ? { selectedItemId: deviceSignInUrlId }
      : {}),
    ...(terminal
      ? {
          removeItemIds: [
            deviceSignInUrlId,
            deviceSignInCodeId,
            deviceSignInRetryId,
            deviceSignInCancelId,
          ],
        }
      : {}),
  });
}

function updateDeviceSignInIndicator(
  ctx: any,
  status: NevermindDeviceSignInStatus,
) {
  if (status.state === 'starting') {
    ctx.ui.indicator.update({
      id: LOGIN_INDICATOR_ID,
      title: 'Nevermind Account',
      subtitle: 'Requesting a one-time sign-in code…',
      status: 'loading',
      durationMs: 300_000,
    });
    return;
  }
  if (status.state === 'pending') {
    ctx.ui.indicator.update({
      id: LOGIN_INDICATOR_ID,
      title: 'Nevermind Account',
      subtitle: status.browserOpenFailed
        ? 'Browser could not be opened. Use the URL and code in the sign-in view.'
        : 'Complete sign-in in your browser',
      status: status.browserOpenFailed ? 'error' : 'loading',
      durationMs: status.browserOpenFailed ? 4000 : 300_000,
    });
    return;
  }
  if (status.state === 'approved') {
    ctx.ui.indicator.update({
      id: LOGIN_INDICATOR_ID,
      title: 'Nevermind Account',
      subtitle: `Logged in as ${status.email}`,
      status: 'success',
      durationMs: 2200,
    });
    return;
  }
  ctx.ui.indicator.update({
    id: LOGIN_INDICATOR_ID,
    title: 'Nevermind Account',
    subtitle: deviceSignInStatusSubtitle(status),
    status: status.state === 'cancelled' ? 'success' : 'error',
    durationMs: 4000,
  });
}

function startDeviceSignIn(ctx: any) {
  ctx.ui.indicator.show({
    id: LOGIN_INDICATOR_ID,
    title: 'Nevermind Account',
    subtitle: 'Requesting a one-time sign-in code…',
    status: 'loading',
    durationMs: 300_000,
  });
  const unsubscribe = extensionContext.onNevermindDeviceSignInChanged(
    (status) => {
      if (!status) return;
      patchDeviceSignInView(ctx, status);
      updateDeviceSignInIndicator(ctx, status);
      extensionContext.invalidateExtensionRootItems();
    },
  );
  const operation = extensionContext.signInToNevermind();
  void operation.then(
    (result) => {
      unsubscribe();
      extensionContext.invalidateExtensionRootItems();
      if (result.ok)
        extensionContext.broadcastAuthChanged({
          authed: true,
          email: result.auth.email,
        });
    },
    () => {
      unsubscribe();
      extensionContext.invalidateExtensionRootItems();
    },
  );
  return {
    view: deviceSignInView(
      ctx,
      extensionContext.getNevermindDeviceSignInState() || {
        state: 'starting',
      },
    ),
    navigation: 'push',
  };
}

export function accountRootItem() {
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
        __handler: async (ctx: any) => {
          const { revoked } = await signOutFromNevermind();
          extensionContext.setActiveNevermindBaseUrl(null);
          await extensionContext.nevermindAi?.disposeAllSessions?.();
          extensionContext.invalidateExtensionRootItems();
          extensionContext.broadcastAuthChanged({ authed: false });
          const suffix = revoked
            ? ''
            : ' (token revoke failed - check connection)';
          showExtensionFeedback(
            ctx,
            'Nevermind Account',
            `Logged out of ${existing.email}${suffix}`,
            revoked ? 'success' : 'error',
          );
        },
      },
    };
  }
  const signInStatus = extensionContext.getNevermindDeviceSignInState();
  const signInInProgress =
    signInStatus?.state === 'starting' || signInStatus?.state === 'pending';
  return {
    id: 'account-login',
    actionId: 'account-login',
    title: signInInProgress
      ? 'Complete sign-in to Nevermind'
      : 'Log in to Nevermind',
    subtitle: signInInProgress
      ? deviceSignInStatusSubtitle(signInStatus)
      : signInStatus
        ? deviceSignInStatusSubtitle(signInStatus)
        : 'Connect this device to your Nevermind account',
    icon: 'person',
    score: 18,
    aliases: ['login', 'sign in', 'nevermind', 'account', 'connect'],
    primaryAction: {
      type: 'runExtensionAction',
      title: signInInProgress ? 'View sign-in status' : 'Log in',
      __handler: (ctx: any) => {
        const currentStatus = extensionContext.getNevermindDeviceSignInState();
        if (
          currentStatus?.state === 'starting' ||
          currentStatus?.state === 'pending'
        )
          return {
            view: deviceSignInView(ctx, currentStatus),
            navigation: 'push',
          };
        return startDeviceSignIn(ctx);
      },
    },
  };
}

export function createAccountExtension() {
  const extensionId = 'nevermind.account';

  function backendEnvironmentItem() {
    async function switchBackend(
      ctx: any,
      input: {
        environment: 'development' | 'production' | 'pr_preview' | 'custom';
        baseUrl?: string;
      },
    ) {
      const result =
        await extensionContext.switchNevermindBackendEnvironment(input);
      showExtensionFeedback(
        ctx,
        'Nevermind Backend',
        result.message,
        result.ok ? 'success' : 'error',
      );
    }
    const developmentAction = {
      type: 'runExtensionAction',
      title: 'Development',
      __handler: async (ctx: any) =>
        switchBackend(ctx, { environment: 'development' }),
    };
    const productionAction = {
      type: 'runExtensionAction',
      title: 'Production',
      __handler: async (ctx: any) =>
        switchBackend(ctx, {
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
          ctx: any,
          action: { formValues?: { baseUrl?: string } },
        ) =>
          switchBackend(ctx, {
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
          ctx: any,
          action: { formValues?: { baseUrl?: string } },
        ) =>
          switchBackend(ctx, {
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
        __handler: async (ctx: any) => {
          await clearByoKey();
          extensionContext.invalidateExtensionRootItems();
          showExtensionFeedback(
            ctx,
            'Nevermind Account',
            'BYO provider key cleared. Using Nevermind backend.',
            'success',
          );
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
        accountRootItem(),
        backendEnvironmentItem(),
        backendStatusItem(),
      ];
      const byo = byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
    rootItems: () => {
      const items = [
        accountRootItem(),
        backendEnvironmentItem(),
        backendStatusItem(),
      ];
      const byo = byoKeyItem();
      if (byo) items.push(byo);
      return items;
    },
  };
}
