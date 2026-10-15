import {
  DEFAULT_AI_CHAT_MODEL,
  normalizeAiChatModel,
} from '../../shared/ai-chat-model';
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
  return {
    recents: loaded.recents || {},
    aliases: loaded.aliases || {},
    shortcuts: loaded.shortcuts || {},
    shortcutActions: loaded.shortcutActions || {},
    removedShortcuts: loaded.removedShortcuts || {},
    overrides: loaded.overrides || {},
    clipboardHistory: loaded.clipboardHistory || [],
    aiChats: loaded.aiChats || {},
    aiChatDefaultModel: normalizeAiChatModel(loaded.aiChatDefaultModel),
    hasCompletedOnboarding:
      typeof loaded.hasCompletedOnboarding === 'boolean'
        ? loaded.hasCompletedOnboarding
        : true,
    settings: loaded.settings || {},
    jobSettings: loaded.jobSettings || {},
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
