import { accountRootItem } from './account';
import { formatShortcut } from '../shared/shortcut-utils';
import { extensionContext } from './_context';

export interface GettingStartedDependencies {
  getUserState: () => Record<string, unknown>;
  getAccountRootItem: () => any;
  getPaletteHotkey: () => string;
  scheduleSaveState: () => void;
  invalidateRootItems: () => void;
  matchesSearch: (item: any, query: string) => boolean;
}

export function createGettingStartedExtension(
  overrides: Partial<GettingStartedDependencies> = {},
) {
  const dependencies: GettingStartedDependencies = {
    getUserState: () => extensionContext.userState,
    getAccountRootItem: accountRootItem,
    getPaletteHotkey: () => String(extensionContext.getPaletteHotkey() || ''),
    scheduleSaveState: () => extensionContext.scheduleSaveState(),
    invalidateRootItems: () => extensionContext.invalidateExtensionRootItems(),
    matchesSearch: (item, query) => extensionContext.rankAction(item, query),
    ...overrides,
  };

  function hasCompletedOnboarding() {
    return dependencies.getUserState().hasCompletedOnboarding === true;
  }

  function completeOnboarding() {
    const userState = dependencies.getUserState();
    if (userState.hasCompletedOnboarding === true) return;
    userState.hasCompletedOnboarding = true;
    dependencies.scheduleSaveState();
    dependencies.invalidateRootItems();
  }

  function dismissWelcomeAction(ctx: any) {
    return ctx.actions.run('Dismiss welcome', async () => {
      completeOnboarding();
    });
  }

  function finishGuideAction(ctx: any) {
    return ctx.actions.run('Done for now', async (actionCtx: any) => {
      completeOnboarding();
      return actionCtx.navigation.pop();
    });
  }

  function signInHelpItem() {
    const account = dependencies.getAccountRootItem();
    if (account?.id !== 'account-login')
      return {
        id: 'getting-started-account',
        title: 'Nevermind account',
        subtitle:
          'You are already signed in. Account access is only needed for AI chats.',
        icon: 'person',
        disabled: true,
      };

    return {
      id: 'getting-started-sign-in',
      title: 'Sign in to Nevermind',
      subtitle: 'Optional: connect your account to use AI chats.',
      icon: 'person',
      primaryAction: { ...account.primaryAction, title: 'Sign in' },
    };
  }

  function paletteShortcutItem(ctx: any) {
    const shortcut = dependencies.getPaletteHotkey().trim();
    const isConfigured = Boolean(shortcut);
    return {
      id: 'getting-started-shortcut',
      title: isConfigured
        ? 'Keyboard shortcut set'
        : 'Open Nevermind from anywhere',
      subtitle: isConfigured
        ? `${formatShortcut(shortcut, process.platform)} opens Nevermind from anywhere. You can change it any time.`
        : 'Optional: set a shortcut for instant access. Launching Nevermind from its app icon also opens the palette.',
      icon: 'keyboard',
      primaryAction: ctx.actions.setPaletteShortcut(
        isConfigured ? 'Change shortcut' : 'Set shortcut',
      ),
    };
  }

  function gettingStartedView(ctx: any) {
    const complete = hasCompletedOnboarding();
    return ctx.ui.list({
      type: 'list',
      id: 'nevermind-getting-started',
      title: complete ? 'Help' : 'Getting Started',
      subtitle:
        'Launch the app to open the palette, or set an optional shortcut for instant access.',
      searchBarPlaceholder: 'Search tips',
      items: [
        {
          id: 'getting-started-find',
          title: 'Find a command',
          subtitle:
            'Return to the palette, then search for an app, file, or command.',
          icon: 'search',
          disabled: true,
        },
        {
          id: 'getting-started-run',
          title: 'Run it',
          subtitle:
            'Choose a result and press Enter. Try "Calculator" or "Open Downloads".',
          icon: 'keyboard',
          disabled: true,
        },
        paletteShortcutItem(ctx),
        signInHelpItem(),
        {
          id: 'getting-started-permissions',
          title: 'Permissions when needed',
          subtitle: 'A prompt appears only when a command needs access.',
          icon: 'shield-check',
          disabled: true,
        },
        {
          id: 'getting-started-done',
          title: 'Done for now',
          subtitle: 'Dismiss this guide and keep using the palette.',
          icon: 'check',
          primaryAction: finishGuideAction(ctx),
        },
      ],
    });
  }

  function gettingStartedItem(ctx: any) {
    const complete = hasCompletedOnboarding();
    return {
      id: 'getting-started',
      title: complete ? 'Help' : 'Welcome to Nevermind',
      subtitle: complete
        ? 'Quick tips for finding and running commands'
        : 'Search apps and files, then press Enter. Launch the app to open the palette or set an optional shortcut.',
      aliases: ['help', 'getting started', 'welcome', 'how to use', 'commands'],
      icon: 'sparkles',
      score: complete ? 14 : 90,
      primaryAction: ctx.actions.push(
        complete ? 'Open Help' : 'Get Started',
        gettingStartedView(ctx),
      ),
      ...(!complete ? { actions: [dismissWelcomeAction(ctx)] } : {}),
    };
  }

  return {
    id: 'nevermind.getting-started',
    title: 'Getting Started',
    capabilities: [] as const,
    rootItems(ctx: any) {
      return [gettingStartedItem(ctx)];
    },
    searchItems(ctx: any, query: string) {
      const item = gettingStartedItem(ctx);
      return dependencies.matchesSearch(item, query) ? [item] : [];
    },
  };
}
