import { DEFAULT_AI_CHAT_MODEL } from '../../shared/ai-chat-model';
import {
  getDefaultNevermindBaseUrl,
  nevermindEnvironmentForBaseUrl,
} from '../backend/nevermind-auth';
import type { UserState } from './user-state';
import type { UserStateRecord } from './user-state-store';

function createDefaultUserState(): UserStateRecord {
  return {
    recents: {},
    aliases: {},
    shortcuts: {},
    shortcutActions: {},
    removedShortcuts: {},
    overrides: {},
    clipboardHistory: [],
    aiChats: {},
    aiChatDefaultModel: DEFAULT_AI_CHAT_MODEL,
    hasCompletedOnboarding: false,
    settings: {},
    jobSettings: {},
    rateCache: {},
    appFirstSeenAtById: {},
    appFirstSeenInitialized: false,
    extensionManager: { schemaVersion: 1, files: {}, proposals: {} },
    nevermindEnvironment: {
      environment: nevermindEnvironmentForBaseUrl(getDefaultNevermindBaseUrl()),
      baseUrl: getDefaultNevermindBaseUrl(),
    },
  };
}

function userStateFromLoadedFile(loaded: UserState): UserStateRecord {
  const defaults = createDefaultUserState();
  return {
    recents: loaded.recents || {},
    aliases: defaults.aliases,
    shortcuts: defaults.shortcuts,
    shortcutActions: loaded.shortcutActions || {},
    removedShortcuts: defaults.removedShortcuts,
    overrides: loaded.overrides || {},
    clipboardHistory: loaded.clipboardHistory || [],
    aiChats: loaded.aiChats || {},
    aiChatDefaultModel: defaults.aiChatDefaultModel,
    hasCompletedOnboarding:
      typeof loaded.hasCompletedOnboarding === 'boolean'
        ? loaded.hasCompletedOnboarding
        : true,
    settings: defaults.settings,
    jobSettings: defaults.jobSettings,
    rateCache: loaded.rateCache || {},
    appFirstSeenAtById: loaded.appFirstSeenAtById || {},
    appFirstSeenInitialized: loaded.appFirstSeenInitialized === true,
    extensionManager: loaded.extensionManager || {
      schemaVersion: 0,
      files: {},
      proposals: {},
    },
    nevermindEnvironment: loaded.nevermindEnvironment || {
      environment: nevermindEnvironmentForBaseUrl(getDefaultNevermindBaseUrl()),
      baseUrl: getDefaultNevermindBaseUrl(),
    },
  };
}

export { createDefaultUserState, userStateFromLoadedFile };
